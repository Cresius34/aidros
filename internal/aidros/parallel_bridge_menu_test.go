package aidros

// Le menu du pont, côté interface (19/11/2026).
//
// La demande de l'humain : « faudrait que ce soit des paramètres ajustables dans
// la section du menu pont ajean ajean ». Le menu réglait déjà le pont et l'agent ;
// il manquait le pilotage, et surtout le filet qui prouve qu'un champ du JSON est
// bien câblé.
//
// Ces tests passent par handleParallelBridge, la route de l'interface, et non par
// les fonctions de réglage. Vérifier les fonctions seules laisserait passer un
// champ absent du `var req` : le POST répond 200, rien n'est écrit, et la case
// coche sans effet. C'est exactement la panne que l'on cherche à ne pas avoir.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// pontPost : un POST sur la route de l'interface, et l'enveloppe rendue.
func pontPost(t *testing.T, corps string) map[string]any {
	t.Helper()
	w := httptest.NewRecorder()
	handleParallelBridge(w, httptest.NewRequest("POST", "http://127.0.0.1/api/bridge", strings.NewReader(corps)))
	if w.Code != http.StatusOK {
		t.Fatalf("POST %s : %d\n%s", corps, w.Code, w.Body.String())
	}
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("enveloppe de %s : %v\n%s", corps, err, w.Body.String())
	}
	return out
}

// TestMenuPontOuvreLePilotage : la case pilotage de la carte écrit bien le
// réglage, dans les deux sens, et le poste le rend.
func TestMenuPontOuvreLePilotage(t *testing.T) {
	parallelTestHome(t)

	// Éteint par défaut : un voisin ne pilote pas d'office cette machine.
	if parallelBridgeControlOn() {
		t.Fatal("le pilotage n'est pas éteint par défaut")
	}
	if r := pontPost(t, `{"control":true}`); r["control_on"] != true {
		t.Fatalf("control:true rendu par le poste ne l'allume pas : %v", r["control_on"])
	}
	if !parallelBridgeControlOn() {
		t.Fatal("parallelBridgeControlOn() toujours faux après un control:true accepté")
	}

	if r := pontPost(t, `{"control":false}`); r["control_on"] != false {
		t.Fatalf("control:false n'a rien coupé : %v", r["control_on"])
	}
	if parallelBridgeControlOn() {
		t.Fatal("le pilotage est toujours ouvert après un control:false")
	}
}

// TestMenuPontPortesIndependantes : ouvrir le pilotage ne doit ouvrir ni le pont
// ni l'agent. Les confondre ferait du pilotage un interrupteur général, ce que la
// gravité de l'acte refuse — il peut éteindre le moteur de la machine.
func TestMenuPontPortesIndependantes(t *testing.T) {
	parallelTestHome(t)

	if err := setParallelBridgeRequested(false); err != nil {
		t.Fatal(err)
	}
	if err := setParallelBridgeAgentOn(false); err != nil {
		t.Fatal(err)
	}

	pontPost(t, `{"control":true}`)

	if parallelBridgeRequested() {
		t.Fatal("ouvrir le pilotage a ouvert le pont : les portes ne sont plus indépendantes")
	}
	if parallelBridgeAgentOn() {
		t.Fatal("ouvrir le pilotage a ouvert l'agent : les portes ne sont plus indépendantes")
	}
}

// TestMenuPontRendLesSources : la carte affiche les SOURCES de l'agent, pas
// seulement son effet. Une case « agent du voisin » allumée alors que le mode
// Agent de la machine est éteint ne déclenche rien (mesuré, parallel_bridge.go) :
// afficher les deux sources évite une case qui ment.
func TestMenuPontRendLesSources(t *testing.T) {
	parallelTestHome(t)

	w := httptest.NewRecorder()
	handleParallelBridge(w, httptest.NewRequest("GET", "http://127.0.0.1/api/bridge", nil))
	var etat map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &etat); err != nil {
		t.Fatal(err)
	}
	for _, cle := range []string{"agent_requested", "agent_mode_local", "agent_on", "control_on"} {
		if _, ok := etat[cle]; !ok {
			t.Fatalf("GET /api/bridge ne rend pas %q : la carte ne peut pas l'afficher", cle)
		}
	}
	// agent_on est le ET des deux sources, pas une troisième valeur inventée.
	if etat["agent_on"].(bool) != (etat["agent_requested"].(bool) && etat["agent_mode_local"].(bool)) {
		t.Fatalf("agent_on=%v ne correspond pas au ET de agent_requested=%v et agent_mode_local=%v",
			etat["agent_on"], etat["agent_requested"], etat["agent_mode_local"])
	}
}

// TestMenuPontNeDevineJamais : un champ absent ne doit rien écrire. « Pas demandé »
// et « faux » sont deux choses, d'où le *bool côté Go ; un bool simple eût coupé
// le pilotage à chaque fois que la carte envoie autre chose (le port, par exemple).
func TestMenuPontNeDevineJamais(t *testing.T) {
	parallelTestHome(t)

	if err := setParallelBridgeControlOn(true); err != nil {
		t.Fatal(err)
	}
	pontPost(t, `{}`)
	if !parallelBridgeControlOn() {
		t.Fatal("un POST sans le champ control a coupé le pilotage : il a deviné « faux »")
	}

	// Pare-côté pour l'agent, la porte déjà en service : même règle.
	if err := setParallelBridgeAgentOn(true); err != nil {
		t.Fatal(err)
	}
	pontPost(t, `{}`)
	if !parallelBridgeAgentOn() {
		t.Fatal("un POST sans le champ agent a coupé l'agent : la règle ne tient pas")
	}
}
