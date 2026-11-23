package aidros

// La reprise d'indexation après une panne de l'encodeur : son filet.
//
// Le défaut vécu, sur le poste de Julien le 23/11 : l'indexation s'arrêtait à
// 1316/1448 avec « Post http://127.0.0.1:18081/v1/embeddings : read tcp …
// wsarecv : Une connexion existante a dû être fermée par l'hôte distant », et NE
// REPARTAIT PAS. 90 s d'échantillon : 0 page, running false, l'erreur rendue
// comme une panne. L'encodeur était mort ; rien ne le relançait.
//
// Filet de CABLAGE, comme celui du veilleur : il relit la source et refuse que la
// boucle d'indexation parle directement à l'encodeur. C'est la seule façon de
// prouver la reprise sans lancer un vrai llama-server deux fois.

import (
	"os"
	"strings"
	"testing"
)

// marqueReprise est la signature de la fonction de reprise. Le filet la cherche
// dans le source : c'est le câblage qui est sous garde, pas un cas d'école.
const marqueReprise = "reprendsEncodageAvecUnEncodeurNeuf"

func sourceIndexation(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("mem_embeddings.go")
	if err != nil {
		t.Fatalf("relire mem_embeddings.go : %v", err)
	}
	return string(b)
}

// sourceReprise relit la fonction de reprise, jusqu'à son accolade fermante en
// début de ligne.
func sourceReprise(t *testing.T) string {
	t.Helper()
	src := sourceIndexation(t)
	i := strings.Index(src, "func "+marqueReprise)
	if i < 0 {
		t.Fatalf("%s est absente de mem_embeddings.go : la reprise n'existe pas", marqueReprise)
	}
	fin := strings.Index(src[i:], "\n}")
	if fin < 0 {
		t.Fatal("la fonction de reprise n'a pas d'accolade fermante en début de ligne")
	}
	return src[i : i+fin]
}

// corpsBoucleIsolation extrait la boucle qui encode les fragments, et RIEN
// d'autre. Sans ce bornage, une marque cherchée « dans le fichier » se
// satisferait de la fonction de reprise elle-même et le filet serait vert sans
// rien contrôler : c'est le faux vert que je me défends de refaire.
func corpsBoucleIsolation(t *testing.T) string {
	t.Helper()
	src := sourceIndexation(t)
	i := strings.Index(src, "for i, c := range chunks {")
	if i < 0 {
		t.Fatal("la boucle d'indexation est introuvable : le fichier a changé de forme")
	}
	fin := strings.Index(src[i:], "\n\t}\n\t// Only publish")
	if fin < 0 {
		fin = strings.Index(src[i:], "\n\tcurrent, err :=")
	}
	if fin < 0 {
		t.Fatal("la fin de la boucle d'indexation est introuvable")
	}
	return src[i : i+fin]
}

func TestRepriseIndexationEncodage(t *testing.T) {
	t.Run("la boucle ne parle plus directement à l'encodeur", func(t *testing.T) {
		boucle := corpsBoucleIsolation(t)
		if strings.Contains(boucle, "client.embedInput(") {
			t.Error("la boucle rappelle embedInput en direct : un encodeur mort arrête l'indexation au lieu de la reprendre")
		}
		if !strings.Contains(boucle, marqueReprise+"(") {
			t.Errorf("la boucle n'appelle pas %s : la reprise n'est pas câblée", marqueReprise)
		}
	})

	t.Run("la reprise ne passe pas par l'arret, qui jetterait les vecteurs", func(t *testing.T) {
		f := sourceReprise(t)
		// stopEmbeddingServer appelle clearEmbeddingCache, qui augmente Epoch. La
		// boucle teste Epoch à chaque fragment : l'appeler ici interdirait au job
		// de finir (« indexation annulée ») et jetterait le travail déjà encodé.
		if strings.Contains(f, "stopEmbeddingServer(") {
			t.Error("la reprise appelle stopEmbeddingServer : elle effacerait les vecteurs déjà encodés et annulerait le job en cours")
		}
		if !strings.Contains(f, "startEmbeddingServer(") {
			t.Error("la reprise n'appelle pas startEmbeddingServer : rien ne relance l'encodeur mort")
		}
	})

	t.Run("la reprise rejoue la requete sur le client neuf", func(t *testing.T) {
		f := sourceReprise(t)
		if !strings.Contains(f, "actif.embedInput(ctx, input)") {
			t.Error("la reprise ne rejoue pas la requête sur le client rendu par le redémarrage")
		}
		if !strings.Contains(f, "return v, actif, nil") {
			t.Error("la reprise ne rend pas le client neuf : les fragments suivants reparleraient à l'encodeur mort")
		}
	})

	t.Run("l'annulation n'est pas une panne", func(t *testing.T) {
		f := sourceReprise(t)
		if !strings.Contains(f, "ctx.Err()") {
			t.Error("la reprise ne distingue pas l'annulation de la panne : un verrouillage ou une échéance relancerait l'encodeur pour rien")
		}
	})

	t.Run("l'echec du redemarrage est nomme, pas masque", func(t *testing.T) {
		f := sourceReprise(t)
		if !strings.Contains(f, "redémarrage de l'encodeur impossible") {
			t.Error("l'échec du redémarrage n'est pas nommé : la panne rendrait un message muet, sans la raison")
		}
	})

	t.Run("la signature rend le client, pas seulement les vecteurs", func(t *testing.T) {
		src := sourceIndexation(t)
		i := strings.Index(src, "func "+marqueReprise)
		if i < 0 {
			t.Fatalf("%s absente", marqueReprise)
		}
		// ⚠ Index sur src[i:] rend une position RELATIVE à i : la tranche doit
		// additionner i, sinon elle régresse avant le début et panique.
		fin := strings.Index(src[i:], "\n")
		if fin < 0 {
			t.Fatal("la signature de la reprise n'a pas de fin de ligne")
		}
		ligne := src[i : i+fin]
		for _, attendu := range []string{"ctx context.Context", "cfg embeddingConfig", "client *embeddingHTTPClient", "input embeddingInput"} {
			if !strings.Contains(ligne, attendu) {
				t.Errorf("signature amputée de %s : %s", attendu, ligne)
			}
		}
		if !strings.Contains(ligne, "([][]float32, *embeddingHTTPClient, error)") {
			t.Errorf("la reprise ne rend pas le client en plus des vecteurs : %s", ligne)
		}
	})
}

// La morsure, jouée dans l'autre sens. embedInput doit rester appelé — mais en
// UN seul endroit : la reprise. Si la boucle retrouvait son appel direct, le
// compte monterait à 2 et ce sous-test rougirait avec le premier. Sans cette
// moitié, le filet pourrait être vert sur un texte qui survivrait au recul.
func TestRepriseIndexationEncodageMorsure(t *testing.T) {
	src := sourceIndexation(t)
	n := strings.Count(src, "client.embedInput(")
	if n != 1 {
		t.Errorf("embedInput appelé %d fois, attendu 1 seul (dans la reprise) : le direct est réapparu", n)
	}
	if !strings.Contains(src, marqueReprise+"(ctx, cfg, client, input)") {
		t.Error("la reprise n'est pas appelée avec le client en place : elle ne peut pas le remplacer")
	}
	// ⚠ Borné à la boucle : la reprise, elle, a bien le droit d'appeler
	// client.embedInput en premier essai. Cherché dans tout le fichier, ce garde
	// rugissait sur le correctif lui-même — rouge joué, pas deviné.
	boucle := corpsBoucleIsolation(t)
	if strings.Contains(boucle, "client.embedInput(") {
		t.Error("l'appel direct est revenu dans la boucle : le défaut du 23/11 est de nouveau possible")
	}
}
