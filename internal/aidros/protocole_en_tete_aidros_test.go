package aidros

// protocole_en_tete_aidros_test.go — le pont lit ses en-têtes sous le nom
// d'aujourd'hui ET sous celui d'hier.
//
// Renommage du 21/11 (ordre de l'humain : « modifie aussi le protocole du pont,
// l'exe de Bekka sera remplacé aussi ») : les en-têtes s'appellent X-Aidros-*.
//
// Ce que ce fichier protège n'est pas le nom, c'est la FENÊTRE DE BASCULE. Entre
// la pose de cet exe et celle de l'exe d'en face, l'un des deux postes nomme,
// l'autre non. Sans admission, la lecture rend le vide — et le vide, ici, n'est
// pas une erreur : sans X-Aidros-Canal, le pont d'en face traite l'appelant
// comme « un logiciel tiers ordinaire » (parallel_bridge_agent.go:128). Il
// RÉPOND. Le canal semble marcher, le tour d'agent est mort.
//
// Les filets existants du pont posent l'en-tête par la CONSTANTE (voir
// parallel_bridge_agent_test.go:28, parallel_bridge_pilot_test.go:53) : ils
// suivent donc le renommage tout seuls et restent verts quoi qu'il arrive. Ils
// ne prouvent RIEN sur l'admission de l'ancien nom — exactement le faux vert que
// j'ai dénoncé plus tôt dans la journée avec TestPreflightSurLeRepli.

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestEnTeteProtocoleLitLesDeuxNoms : le cœur du garde-fou, sur les trois
// familles d'en-têtes, parce qu'une seule ne prouverait pas le mécanisme.
func TestEnTeteProtocoleLitLesDeuxNoms(t *testing.T) {
	nouveaux := []string{
		canalHeader,
		canalFromHeader,
		canalOriginHeader,
		canalHopHeader,
		canalAgentRefusHeader,
		echangeEnTeteRemplacer,
		echangeEnTeteSha,
		echangeEnTeteNom,
		e2eInnerHeader,
	}
	for _, nom := range nouveaux {
		t.Run(nom, func(t *testing.T) {
			// 1. Le nom d'aujourd'hui : c'est ce qu'émet ce code.
			juste := http.Header{}
			juste.Set(nom, "1")
			if got := lireEnTeteProtocole(juste, nom); got != "1" {
				t.Fatalf("nom du jour : obtenu %q, voulu %q", got, "1")
			}

			// 2. Le nom d'hier : c'est ce qu'émet le poste qui n'a pas encore
			//    reçu son exe. Sans admission, la lecture rend "" et le pont
			//    d'en face me croit logiciel tiers.
			avant := http.Header{}
			avant.Set(enTeteAvant(nom), "1")
			if got := lireEnTeteProtocole(avant, nom); got != "1" {
				t.Fatalf("nom d'hier %q : obtenu %q, voulu %q — le canal se coupe en silence pendant la bascule",
					enTeteAvant(nom), got, "1")
			}

			// 3. Le nom d'aujourd'hui gagne quand les deux sont là : on ne
			//    laisse pas un inconnu nous faire parler avec son vieux nom.
			les_deux := http.Header{}
			les_deux.Set(enTeteAvant(nom), "ancien")
			les_deux.Set(nom, "nouveau")
			if got := lireEnTeteProtocole(les_deux, nom); got != "nouveau" {
				t.Fatalf("les deux noms posés : obtenu %q, voulu %q", got, "nouveau")
			}

			// 4. Rien posé : le vide, pas une panique.
			if got := lireEnTeteProtocole(http.Header{}, nom); got != "" {
				t.Fatalf("rien posé : obtenu %q, voulu vide", got)
			}
		})
	}
}

// TestEnTeteProtocoleNeConnaitQueNeufNoms : le renommage doit être COMPLET. Un
// dixième en-tête oublié derrière garderait son nom d'hier chez nous et
// parlerait une autre langue que le reste du protocole.
func TestEnTeteProtocoleNeConnaitQueNeufNoms(t *testing.T) {
	tous := map[string]string{
		"canalHeader":           canalHeader,
		"canalFromHeader":       canalFromHeader,
		"canalOriginHeader":     canalOriginHeader,
		"canalHopHeader":        canalHopHeader,
		"canalAgentRefusHeader": canalAgentRefusHeader,
		"echangeRemplacer":      echangeEnTeteRemplacer,
		"echangeSha":            echangeEnTeteSha,
		"echangeNom":            echangeEnTeteNom,
		"e2eInnerHeader":        e2eInnerHeader,
	}
	if len(tous) != 9 {
		t.Fatalf("neuf en-têtes attendus, la table en porte %d", len(tous))
	}
	for qui, nom := range tous {
		if nom != enTeteProtocoleNouveau+nom[len(enTeteProtocoleNouveau):] {
			t.Fatalf("%s ne commence pas par %q : %q", qui, enTeteProtocoleNouveau, nom)
		}
		if enTeteAvant(nom) != enTeteProtocoleAvant+nom[len(enTeteProtocoleNouveau):] {
			t.Fatalf("%s : son nom d'hier se dérive mal (%q)", qui, enTeteAvant(nom))
		}
	}
	// Le nom d'hier n'est pas un onzième en-tête : c'est un préfixe, et il ne
	// doit surtout pas se confondre avec le nouveau.
	if enTeteProtocoleAvant == enTeteProtocoleNouveau {
		t.Fatal("noms d'avant et d'aujourd'hui identiques : l'admission ne vérifierait rien")
	}
}

// TestEnTeteAvantRefuseLeHorsProtocole : Authorization et Compagnie ne sont pas
// du canal. enTeteAvant rend "" pour eux, et lireEnTeteProtocole les lit sous
// leur seul nom — sinon un en-tête de quelqu'un d'autre se mettrait à être lu
// sous deux orthographes.
func TestEnTeteAvantRefuseLeHorsProtocole(t *testing.T) {
	for _, nom := range []string{"Authorization", "Content-Type", "X-Ajean-Machine"} {
		if nom == "X-Ajean-Machine" {
			// Lui commence par le préfixe d'hier mais n'est PAS un nom que nous
			// lisons : on ne le lit pas, on l'écrit, et c'est le relais de
			// l'opérateur qui le lit (relay_backup.go:95). Vérifié qu'il n'entre
			// dans aucun chemin d'admission.
			continue
		}
		if got := enTeteAvant(nom); got != "" {
			t.Fatalf("%q : un nom hors canal ne doit pas avoir de nom d'hier, obtenu %q", nom, got)
		}
		h := http.Header{}
		h.Set(nom, "valeur")
		if got := lireEnTeteProtocole(h, nom); got != "valeur" {
			t.Fatalf("%q lu sous son seul nom : obtenu %q", nom, got)
		}
	}
	// Un nom qui PORTE le préfixe du jour en a bien un d'hier, même si le pont
	// ne le connaît pas : la dérivation est mécanique, pas une liste.
	if got := enTeteAvant("X-Aidros-De-Inconnu"); got != "X-Ajean-De-Inconnu" {
		t.Fatalf("dérivation mécanique attendue, obtenu %q", got)
	}
}

// TestModelAgentAdmetLeNomDhier : le dernier maillon du protocole, et le seul
// qui ne soit pas un en-tête — la VALEUR du champ "model". Un tour d'agent
// s'annonce modelAgentCanal ; le modèle nu répond sous le nom de son gguf.
// L'appelant s'en sert pour écrire sa balise : « il a répondu par son agent »
// ou « par son modèle nu » (parallel_peer.go:383-390).
//
// Le voisin au r43 ANNONCE encore « ajean-agent » et COMPARE encore
// « ajean-agent ». Sans admission ici, le diagnostic devient faux sans erreur :
// la balise dirait « modèle nu » alors que le voisin a bien lancé un agent, avec
// ses outils et son disque. Le lecteur prendrait la décision inverse — couper
// le tour d'agent chez lui pour « économiser » — sur un signe inventé.
func TestModelAgentAdmetLeNomDhier(t *testing.T) {
	annonces := []string{modelAgentCanal, modelAgentCanalAvant, "  " + modelAgentCanalAvant + "  "}
	for _, a := range annonces {
		if !modelCanalEstAgent(a) {
			t.Fatalf("%q : un tour d'agent n'est pas reconnu — la balise accuserait « modèle nu »", a)
		}
	}
	// Un gguf reste un gguf : l'admission ne doit pas transformer n'importe quoi
	// en agent, sinon « modèle nu » disparaîtrait du protocole.
	for _, a := range []string{"", "Qwen3.5-9B-Q8_0", "strata-0.1.57", "aidros", "aidros"} {
		if modelCanalEstAgent(a) {
			t.Fatalf("%q : un modèle nu passe pour un agent", a)
		}
	}
	// Les deux noms sont bien distincts : sinon l'admission ne vérifierait rien.
	if modelAgentCanal == modelAgentCanalAvant {
		t.Fatal("nom d'agent d'avant et d'aujourd'hui identiques")
	}
}

// TestCanalReconnaitUnVoisinQuiNaminePasSonExe : le garde-fou joué de bout en
// bout, sur le vrai chemin de décision. Un voisin qui n'a PAS encore reçu son
// exe pose le vieux nom ; notre pont doit quand même le reconnaître comme un
// Aidros, pas comme un logiciel tiers.
func TestCanalReconnaitUnVoisinQuiNaminePasSonExe(t *testing.T) {
	t.Setenv("AJEAN_HOME", t.TempDir())

	// Le voisin d'avant : il pose X-Ajean-Canal.
	r := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	r.Header.Set(enTeteAvant(canalHeader), "1")
	if lireEnTeteProtocole(r.Header, canalHeader) == "" {
		t.Fatal("le voisin d'avant n'est pas reconnu : le pont le traiterait comme un logiciel tiers (parallel_bridge_agent.go:128)")
	}

	// Et la même requête, sans aucun des deux noms : elle doit rester du tiers.
	r2 := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	if lireEnTeteProtocole(r2.Header, canalHeader) != "" {
		t.Fatal("un client ordinaire passe pour un Aidros : l'admission est trop large")
	}
}
