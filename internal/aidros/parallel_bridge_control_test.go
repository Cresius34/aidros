package aidros

// Le filet du pilotage à distance d'un Ajean par le pont.
//
// Comme celui de l'échange de fichiers (parallel_bridge_files_test.go:5), il
// appelle le HANDLER DU PONT et non handleControl directement. Les deux
// placements fautifs de la route sont SILENCIEUX et je les ai mesurés le 17/11
// pour les fichiers : trop bas, la garde /v1/* de parallel_bridge.go:168 répond
// 404 « surface OpenAI uniquement » ; après canalDemande, un POST de pilotage
// part en tour d'AGENT et rend du texte au lieu d'avoir changé quoi que ce soit.
// Dans les deux cas, tester handleControl tout seul resterait VERT.
//
// Il vérifie aussi la DONNÉE : le millésime du moteur, qui est la pièce qui a
// coûté quatre tours de diagnostic chez la voisine le 17/11 (deux presets
// GYRO-S, l'UI affiche le # NAME et non le millésime, elle avait renommé le
// mauvais des deux).

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func pilotageTestRequete(methode, chemin, corps string, entetes map[string]string) *http.Request {
	var lecteur *bytes.Reader = bytes.NewReader(nil)
	if corps != "" {
		lecteur = bytes.NewReader([]byte(corps))
	}
	req := httptest.NewRequest(methode, "http://192.168.1.9"+chemin, lecteur)
	// Depuis le réseau : sinon antKeyOK admet sans clé (anthropic_api.go:302).
	req.RemoteAddr = "192.168.1.42:51515"
	req.Header.Set("Authorization", "Bearer sk-pilotage-test")
	for k, v := range entetes {
		req.Header.Set(k, v)
	}
	return req
}

// TestPilotageSousLaGardeDuPont : la route DOIT vivre sous /v1/. C'est le test
// qui aurait attrapé le 404 de la garde. Il prouve aussi que le POST n'est PAS
// avalé par un tour d'agent : un tour d'agent répondrait 200 avec du texte, ici
// on exige une enveloppe JSON avec le champ attendu.
func TestPilotageSousLaGardeDuPont(t *testing.T) {
	parallelTestHome(t)
	if err := writeAPIKey("sk-pilotage-test"); err != nil {
		t.Fatal(err)
	}
	// ÉTEINT : le pilotage ne se décrète pas tout seul, un voisin ne doit pas
	// pouvoir éteindre mon moteur sans que la machine l'ait accepté.
	if err := setParallelBridgeControlOn(false); err != nil {
		t.Fatal(err)
	}
	h := parallelBridgeHandler()

	// 1) un /api/switch envoyé au pont doit rester écarté par la garde : c'est la
	//    preuve que la route ne peut PAS vivre là, et donc que /v1/ est nécessaire.
	w := httptest.NewRecorder()
	h.ServeHTTP(w, pilotageTestRequete("POST", "/api/switch", `{"id":"X"}`, nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("/api/switch sur le pont : %d, attendu 404 (la garde ne sert que /v1/*)", w.Code)
	}

	// 2) interrupteur éteint : 403 ET le message qui dit comment l'allumer. Un
	//    403 muet ferait chercher midi à quatorze heures du côté de l'appelant.
	w = httptest.NewRecorder()
	h.ServeHTTP(w, pilotageTestRequete("GET", "/v1/presets", "", nil))
	if w.Code != http.StatusForbidden {
		t.Fatalf("/v1/presets interrupteur éteint : %d, attendu 403\n%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "bridge control on") {
		t.Fatalf("le 403 ne dit pas comment ouvrir l'interrupteur :\n%s", w.Body.String())
	}

	// 3) la route est bien AIGUILLÉE : une méthode non admise doit rendre 405,
	//    pas 404 (garde) ni un texte d'agent. C'est CE test qui distingue les
	//    trois placements.
	if err := setParallelBridgeControlOn(true); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, pilotageTestRequete("DELETE", "/v1/presets", "", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("/v1/presets en DELETE : %d, attendu 405 — la route est-elle branchée avant le canal ?\n%s",
			w.Code, w.Body.String())
	}

	// 4) un preset inconnu ne doit JAMAIS redémarrer quoi que ce soit : 404.
	w = httptest.NewRecorder()
	h.ServeHTTP(w, pilotageTestRequete("POST", "/v1/presets/switch", `{"id":"absent-depuis-toujours"}`, nil))
	if w.Code != http.StatusNotFound {
		t.Fatalf("switch sur un preset absent : %d, attendu 404\n%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "introuvable") {
		t.Fatalf("le 404 ne dit pas ce qui est introuvable :\n%s", w.Body.String())
	}
}

// TestPilotageListeEtMillésime : la liste rendue doit porter le MILLÉSIME du
// moteur que chaque preset va chercher. C'est la pièce qui manquait à la voisine
// le 17/11 : elle voyait « FLASH NEXT GYRO-S HIP 1.48 » et « FLASH NEXT GYRO-S
// HIP », deux noms, aucun millésime, et le second était le seul bon.
func TestPilotageListeEtMillésime(t *testing.T) {
	parallelTestHome(t)
	if err := writeAPIKey("sk-pilotage-test"); err != nil {
		t.Fatal(err)
	}
	if err := setParallelBridgeControlOn(true); err != nil {
		t.Fatal(err)
	}

	// Deux presets GYRO-S comme chez elle : même famille, deux millésimes.
	for _, p := range []struct{ fichier, contenu string }{
		{"GYRO-S-MAUVAIS.env", "# NAME=FLASH NEXT GYRO-S HIP 1.48\nENGINE=moe\nMOE_RUNTIME=strata-windows\nMOE_CONFIG=C:\\moe\\strata-0.1.48-hip\\serve\\server.py\nMOE_PYTHON=C:\\moe\\strata-0.1.48-hip\\python\\python.exe\nMOE_SOURCE=C:\\moe\\strata-0.1.48-hip\nPORT=18080\n"},
		{"GYRO-S-BON.env", "# NAME=FLASH NEXT GYRO-S HIP\nENGINE=moe\nMOE_RUNTIME=strata-windows\nMOE_CONFIG=C:\\moe\\strata-0.1.57-hip\\serve\\server.py\nMOE_PYTHON=C:\\moe\\strata-0.1.57-hip\\python\\python.exe\nMOE_SOURCE=C:\\moe\\strata-0.1.57-hip\nPORT=18080\n"},
	} {
		if err := os.WriteFile(filepath.Join(presetsDir(), p.fichier), []byte(p.contenu), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	h := parallelBridgeHandler()
	w := httptest.NewRecorder()
	h.ServeHTTP(w, pilotageTestRequete("GET", "/v1/presets", "", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("/v1/presets : %d\n%s", w.Code, w.Body.String())
	}

	// Décoder l'ENVELOPPE, pas le tableau nu : la leçon de preset_moe_ui_test.go:36.
	var enveloppe struct {
		OK        bool `json:"ok"`
		Presets   []map[string]any
		Moteur    string `json:"moteur_epingle"`
		Attention string `json:"attention"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &enveloppe); err != nil {
		t.Fatalf("enveloppe : %v\n%s", err, w.Body.String())
	}
	if !enveloppe.OK {
		t.Fatalf("ok faux :\n%s", w.Body.String())
	}
	if len(enveloppe.Presets) < 2 {
		t.Fatalf("%d preset(s) rendu(s), attendu au moins 2\n%s", len(enveloppe.Presets), w.Body.String())
	}
	if enveloppe.Attention == "" {
		t.Fatal("la liste ne dit pas qu'un basculement arrête le moteur et coupe le pont")
	}

	parID := map[string]map[string]any{}
	for _, p := range enveloppe.Presets {
		if id, _ := p["id"].(string); id != "" {
			parID[id] = p
		}
	}

	for _, want := range []struct{ id, millésime string }{
		{"GYRO-S-MAUVAIS", "0.1.48"},
		{"GYRO-S-BON", "0.1.57"},
	} {
		p, ok := parID[want.id]
		if !ok {
			t.Fatalf("preset %s absent de la liste rendue", want.id)
		}
		if p["moe"] != true {
			t.Fatalf("%s : moe=%v, la liste ne reconnaît pas un preset MoE\n%v", want.id, p["moe"], p)
		}
		got, _ := p["moteur"].(string)
		if got != want.millésime {
			t.Fatalf("%s : millésime %q, attendu %q — c'est exactement le piège du 17/11",
				want.id, got, want.millésime)
		}
		// Le nom affiché ne dit PAS le millésime : c'est le piège. Si un jour il le
		// disait, ce test ne prouverait plus rien ; on le vérifie donc.
		if nom, _ := p["name"].(string); strings.Contains(nom, want.millésime) {
			t.Logf("avertissement : le nom %q porte déjà le millésime, le test ne prouverait plus rien", nom)
		}
	}

	// Le millésime épinglé par CET exe, pour qu'un voisin ne choisisse pas un
	// preset que mon binaire refusera.
	if enveloppe.Moteur != strataVersion {
		t.Fatalf("moteur_epingle %q, attendu %q", enveloppe.Moteur, strataVersion)
	}
}

// TestPilotageMessageCiteUneCommandeQuiExiste : le 403 rendu au voisin dit
// « allumer : ajean bridge control on ». Un message qui nomme une commande
// inexistante envoie l'humain dans le mur, et c'est le genre de mensonge qu'on
// écrit sans le vouloir. Ce test ferme la boucle : la commande citée dans
// l'erreur est la même que celle qu'accepte la CLI, et elle change bien l'état.
func TestPilotageMessageCiteUneCommandeQuiExiste(t *testing.T) {
	parallelTestHome(t)

	// La commande citée mot pour mot dans le 403 (handleControl). La CLI reçoit
	// les arguments APRES « bridge » : cmdParallelBridge(["control","on"]) pour
	// « ajean bridge control on ». Couper le préfixe « ajean bridge » fait partie
	// du test, sinon on enverrait sub="bridge" et la CLI répondrait autre chose.
	const citée = "ajean bridge control on"
	args := strings.Fields(strings.TrimPrefix(citée, "ajean bridge "))
	if len(args) != 2 || args[0] != "control" {
		t.Fatalf("préfixe mal coupé sur %q -> %q", citée, args)
	}

	if err := setParallelBridgeControlOn(false); err != nil {
		t.Fatal(err)
	}
	if err := cmdParallelBridge(args); err != nil {
		t.Fatalf("la CLI refuse la commande citée dans le 403 (%q) : %v", citée, err)
	}
	if !parallelBridgeControlOn() {
		t.Fatalf("« %s » rendu par la CLI n'allume pas l'interrupteur que le 403 éteint", citée)
	}
	if err := cmdParallelBridge([]string{"control", "off"}); err != nil {
		t.Fatal(err)
	}
	if parallelBridgeControlOn() {
		t.Fatal("« control off » n'a rien coupé")
	}
	// Une valeur floue doit refuser, pas deviner dans le sens le plus dangereux.
	if err := cmdParallelBridge([]string{"control", "peut-etre"}); err == nil {
		t.Fatal("« control peut-etre » accepté : la CLI devine, elle ne devrait pas")
	}
}

// TestPontLaissePasserLesImages : la route des images TRAVERSIE le pont sans
// rien ajouter. oaiHandler (llm_oai.go:67) proxifie tout ce qui commence par
// /v1 vers le moteur, donc /v1/images/generations y passe déjà.
//
// Le test ne prouve pas que la génération marche — le moteur de test n'a pas de
// générateur d'images. Il prouve ce qui compte pour le chantier : le pont
// n'est pas le verrou. Les deux issues se distinguent au code de retour, et le
// moteur est ARRÊTÉ dans ce test, donc :
//   - 502 « llama-server injoignable » = la requête a été ROUTÉE vers le moteur
//     (le pont l'a laissée passer) ;
//   - 404 « surface OpenAI uniquement » = le pont l'a écartée avant le moteur.
//
// C'est 502 qui est attendu.
func TestPontLaissePasserLesImages(t *testing.T) {
	parallelTestHome(t)
	if err := writeAPIKey("sk-pilotage-test"); err != nil {
		t.Fatal(err)
	}
	h := parallelBridgeHandler()

	w := httptest.NewRecorder()
	h.ServeHTTP(w, pilotageTestRequete("POST", "/v1/images/generations",
		`{"prompt":"un renard sur une roche moussue","size":"512x512","n":1,"response_format":"b64_json"}`, nil))

	if w.Code == http.StatusNotFound && strings.Contains(w.Body.String(), "surface OpenAI") {
		t.Fatalf("le pont écarte /v1/images/generations avant le moteur : le chantier image\n"+
			"passe par le pont, il faudra l'ajouter à la liste. Corps : %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "injoignable") {
		t.Fatalf("code %d, corps inattendu — le routage vers le moteur n'est pas prouvé :\n%s",
			w.Code, w.Body.String())
	}
	// 502 avec « injoignable » : la requête est sortie du pont vers le moteur.
	if w.Code != http.StatusBadGateway {
		t.Logf("code %d (attendu 502) mais la preuve est là : %s", w.Code, w.Body.String())
	}
}

// TestPilotageMillésimeDuChemin : la fonction pure, sur les formes réelles.
func TestPilotageMillésimeDuChemin(t *testing.T) {
	for _, c := range []struct{ chemin, attendu string }{
		{`C:\ProgramData\Ajean v20.1.3 maj Cresius RX6900XT\data\moe\strata-0.1.57-hip\serve\server.py`, "0.1.57"},
		{`C:\moe\strata-0.1.48-hip\python\python.exe`, "0.1.48"},
		{`C:\moe\strata-0.1.59-hip`, "0.1.59"},
		{`/home/x/moe/strata-0.2.13-hip/serve/server.py`, "0.2.13"},
		{`C:\moe\strata-0.1.57\serve\server.py`, "0.1.57"}, // sans suffixe -hip
		{`C:\moteurs\llama-b1354-windows\llama-server.exe`, ""},
		{``, ""},
	} {
		if got := moeMillésimeDuChemin(c.chemin); got != c.attendu {
			t.Errorf("moeMillésimeDuChemin(%q) = %q, attendu %q", c.chemin, got, c.attendu)
		}
	}
}

// TestPilotageMode : allumer/couper l'agent du voisin à distance, et lui DIRE si
// la machine tient encore sa propre porte fermée (mesuré : parallel_bridge.go:825).
func TestPilotageMode(t *testing.T) {
	parallelTestHome(t)
	if err := writeAPIKey("sk-pilotage-test"); err != nil {
		t.Fatal(err)
	}
	if err := setParallelBridgeControlOn(true); err != nil {
		t.Fatal(err)
	}
	h := parallelBridgeHandler()

	// Refus d'une valeur qui n'est ni on ni off : ne jamais deviner dans ce sens-là.
	w := httptest.NewRecorder()
	h.ServeHTTP(w, pilotageTestRequete("POST", "/v1/mode", `{"agent":"peut-etre"}`, nil))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("/v1/mode valeur floue : %d, attendu 400\n%s", w.Code, w.Body.String())
	}

	// Les deux en-têtes du canal, posés par peerAsk (parallel_peer.go:308) et
	// peerOrigineCLI (L489). Le poste piloté doit savoir QUI il a en face : la
	// machine, et si c'est une personne ou une IA. C'est l'arbitrage du 27/10.
	w = httptest.NewRecorder()
	h.ServeHTTP(w, pilotageTestRequete("POST", "/v1/mode", `{"agent":"on"}`,
		map[string]string{canalFromHeader: "Cresius", canalOriginHeader: canalOrigineHumain}))
	if w.Code != http.StatusOK {
		t.Fatalf("/v1/mode on : %d\n%s", w.Code, w.Body.String())
	}
	var rendu struct {
		OK        bool   `json:"ok"`
		Agent     bool   `json:"agent"`
		Attention string `json:"attention"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &rendu); err != nil {
		t.Fatalf("enveloppe : %v\n%s", err, w.Body.String())
	}
	if !rendu.OK || !rendu.Agent {
		t.Fatalf("agent pas allumé : %s", w.Body.String())
	}
	if !parallelBridgeAgentOn() {
		t.Fatal("parallelBridgeAgentOn() toujours faux après un on rendu vrai")
	}
	// Le mode agent de la MACHINE n'est pas forcément allumé : dans une base de
	// test il ne l'est pas, donc l'avertissement est dû.
	if !agentEnabled() && rendu.Attention == "" {
		t.Fatalf("le pont vient d'ouvrir une porte que la machine tient fermée, et ne le dit pas :\n%s", w.Body.String())
	}

	w = httptest.NewRecorder()
	h.ServeHTTP(w, pilotageTestRequete("POST", "/v1/mode", `{"agent":"off"}`, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("/v1/mode off : %d\n%s", w.Code, w.Body.String())
	}
	if parallelBridgeAgentOn() {
		t.Fatal("agent toujours allumé après un off")
	}

	// relecture : le voisin doit pouvoir vérifier sans rejouer le geste.
	w = httptest.NewRecorder()
	h.ServeHTTP(w, pilotageTestRequete("GET", "/v1/mode", "", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET /v1/mode : %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `"agent":false`) {
		t.Fatalf("GET /v1/mode ne rend pas l'état coupé :\n%s", w.Body.String())
	}
}

// TestPilotageSansCle : la route de pilotage est sous la GARDE DE CLÉ, pas à
// côté. Un voisin sans la clé ne doit rien pouvoir basculer.
func TestPilotageSansCle(t *testing.T) {
	parallelTestHome(t)
	if err := writeAPIKey("sk-pilotage-test"); err != nil {
		t.Fatal(err)
	}
	if err := setParallelBridgeControlOn(true); err != nil {
		t.Fatal(err)
	}
	h := parallelBridgeHandler()

	req := httptest.NewRequest("POST", "http://192.168.1.9/v1/presets/switch", bytes.NewReader([]byte(`{"id":"X"}`)))
	req.RemoteAddr = "192.168.1.42:51515" // réseau : la boucle locale admet sans clé
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("switch sans clé : %d, attendu 401\n%s", w.Code, w.Body.String())
	}
}
