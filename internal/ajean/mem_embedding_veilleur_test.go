package ajean

// Garde de la voie Python de l'encodeur de mémoire.
//
// Pourquoi ce fichier : la voie « transformers » (Python + torch) était la seule
// à ne subir AUCUNE garde mémoire ni inactivité. Mesuré le 14/11/2026 sur le
// poste 20.1 : indexation finie (0,1 s de CPU sur 25 s), l'encodeur retenait
// 24,8 Go privés / 15,9 Go vifs et ne les rendait pas ; RAM libre tombée à
// 5,5 Go sur 127,1, avec le moteur MoE à 74,9 Go dans le même temps.
// stopEmbeddingServer n'était appelé qu'à la demande (web_embeddings), au
// changement de preset (backend_presets) ou à l'arrêt (parallel_profile) :
// aucune boucle périodique. La voie llama-server, elle, avait déjà watch()
// (jean_embed.go:268-287) sur les mêmes seuils.
//
// La leçon de la soirée du 14/11 est qu'un `go vet` vert et un paquet « ok »
// laissaient passer un moteur mort, parce que l'amont n'a aucun test MoE. Donc
// ce test mord dans les DEUX sens : la décision pure, et le câblage relu dans
// le source (précédent dans ce dépôt : jean_embed_eg2_test.go lit des .go).

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVeilleurEncodeurPython_Decisions(t *testing.T) {
	t0 := time.Date(2026, 11, 14, 20, 0, 0, 0, time.UTC)

	t.Run("sous la tolérance de démarrage, RIEN n'est tué", func(t *testing.T) {
		// torch installe ses dépendances : beaucoup de RAM, aucun appel récent.
		started := t0
		now := t0.Add(embeddingGrace - time.Minute)
		lastUse := t0.Add(-2 * time.Hour)
		if stop, why := embeddingWatchDecision(now, started, lastUse, 120); stop {
			t.Errorf("arrêté pendant la tolérance de démarrage (%s) : %s", embeddingGrace, why)
		}
	})

	t.Run("RAM trop basse apres la tolerance : arrete", func(t *testing.T) {
		started := t0.Add(-2 * embeddingGrace)
		now := t0
		lastUse := now.Add(-time.Second) // vient de servir : seule la RAM décide
		stop, why := embeddingWatchDecision(now, started, lastUse, embeddingStopRAMMB-1)
		if !stop {
			t.Fatalf("RAM à %d Mo : le veilleur ne l'a pas arrêté (seuil %d)", embeddingStopRAMMB-1, embeddingStopRAMMB)
		}
		if !strings.Contains(why, "RAM") {
			t.Errorf("motif sans le mot RAM : %q", why)
		}
	})

	t.Run("RAM juste au-dessus du seuil : on ne tue pas", func(t *testing.T) {
		started := t0.Add(-2 * embeddingGrace)
		lastUse := time.Now().Add(-time.Second)
		if stop, why := embeddingWatchDecision(time.Now(), started, lastUse, embeddingStopRAMMB); stop {
			t.Errorf("arrêté à exactement %d Mo, attendu seulement en dessous : %s", embeddingStopRAMMB, why)
		}
	})

	t.Run("RAM indisponible (-1) : on ne tue pas sur un doute", func(t *testing.T) {
		started := t0.Add(-2 * embeddingGrace)
		lastUse := time.Now().Add(-time.Second)
		if stop, why := embeddingWatchDecision(time.Now(), started, lastUse, -1); stop {
			t.Errorf("arrêté sur une RAM inconnue : %s", why)
		}
	})

	t.Run("inactif depuis plus de la garde : arrete", func(t *testing.T) {
		started := t0.Add(-2 * embeddingGrace)
		now := time.Now()
		lastUse := now.Add(-embeddingIdleStop - time.Minute)
		stop, why := embeddingWatchDecision(now, started, lastUse, 999999)
		if !stop {
			t.Fatalf("inactif depuis %s : le veilleur ne l'a pas arrêté", embeddingIdleStop+time.Minute)
		}
		if !strings.Contains(why, "inutilisé") {
			t.Errorf("motif sans « inutilisé » : %q", why)
		}
	})

	t.Run("indexation en cours : la marque de vie le sauve", func(t *testing.T) {
		// C'est le cas qui protège les vecteurs déjà encodés : un encodage long
		// tient la machine en RAM mais sert l'encodeur, donc lastUse est récent.
		started := t0.Add(-2 * embeddingGrace)
		now := time.Now()
		lastUse := now.Add(-embeddingIdleStop + 30*time.Second)
		if stop, why := embeddingWatchDecision(now, started, lastUse, 999999); stop {
			t.Errorf("arrêté alors que l'encodeur vient de servir : %s", why)
		}
	})

	t.Run("les seuils sont ceux de la voie llama-server", func(t *testing.T) {
		// Les deux voies doivent obéir à la même politique, sinon l'interface
		// mente sur ce qu'« éteindre l'encodeur » veut dire selon le réglage.
		if embeddingStopRAMMB != jeanEmbedStopMB {
			t.Errorf("seuil RAM : encodeur Python %d, llama-server %d", embeddingStopRAMMB, jeanEmbedStopMB)
		}
		if embeddingIdleStop != jeanEmbedIdle {
			t.Errorf("garde d'inactivité : encodeur Python %s, llama-server %s", embeddingIdleStop, jeanEmbedIdle)
		}
	})
}

// sourceBackend relit mem_embedding_backend.go : le câblage du veilleur ne peut
// pas se prouver sans un vrai Python + torch, donc on l'exige dans le texte.
// (Précédent dans ce dépôt : jean_embed_eg2_test.go relit des .go.)
func sourceBackend(t *testing.T) string {
	t.Helper()
	const nom = "mem_embedding_backend.go"
	b, err := os.ReadFile(nom)
	if err != nil {
		t.Fatalf("relire %s : %v", nom, err)
	}
	return string(b)
}

func TestVeilleurEncodeurPython_Cablage(t *testing.T) {
	src := sourceBackend(t)

	t.Run("startEmbeddingServer lance le veilleur", func(t *testing.T) {
		if !strings.Contains(src, "go embeddingWatch(p,") {
			t.Error("aucun go embeddingWatch( : la voie Python n'a toujours pas de veilleur, elle rendra sa RAM à l'arrêt de l'app seulement")
		}
	})

	t.Run("toute requete marque la vie", func(t *testing.T) {
		// embedRequest est le seul point de passage : embed() texte et
		// embedInput() multimodal y aboutissent tous les deux.
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, filepath.Join(".", "mem_embedding_backend.go"), src, 0)
		if err != nil {
			t.Fatalf("parse : %v", err)
		}
		marque := false
		ast.Inspect(f, func(n ast.Node) bool {
			fn, ok := n.(*ast.FuncDecl)
			if !ok || fn.Name == nil || fn.Name.Name != "embedRequest" {
				return true
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if ok {
					if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "markEmbeddingUse" {
						marque = true
					}
				}
				return true
			})
			return false
		})
		if !marque {
			t.Error("embedRequest n'appelle pas markEmbeddingUse : une indexation en cours pourrait être tuée, ses vecteurs déjà encodés perdus")
		}
	})

	t.Run("le veilleur ne se substitue pas a un arret volontaire", func(t *testing.T) {
		// stopEmbeddingServer met embeddingServer.p à nil : le veilleur doit s'en
		// aller sans redemander un arrêt, sinon les deux s'appellent l'un l'autre.
		if !strings.Contains(src, "case <-p.done:") {
			t.Error("le veilleur n'observe pas p.done : il peut tourner après un arrêt volontaire")
		}
		if !strings.Contains(src, "embeddingServer.p == p") {
			t.Error("le veilleur ne vérifie pas qu'il veille toujours sur l'instance en place")
		}
	})

	t.Run("la tolerance de demarrage existe", func(t *testing.T) {
		// Le premier lancement de la voie Python installe torch : tuer pendant
		// cette phase tuerait une installation légitime, en plein travail.
		if embeddingGrace <= embeddingIdleStop {
			t.Errorf("tolérance de démarrage %s <= garde d'inactivité %s : un premier lancement serait tué", embeddingGrace, embeddingIdleStop)
		}
	})
}
