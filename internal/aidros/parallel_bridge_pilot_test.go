package aidros

// Le pilotage sans modèle chargé, et le relais local vers un voisin.
//
// L'exigence posée par l'humain le 19/11 : « faut que ça puisse prendre la main
// sans modèle chargé ». Ce fichier la joue pour de vrai, pas en intention.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPilotageSansModeleCharge : le poste piloté est pilotable MÊME SI aucun
// moteur ne tourne derrière lui.
//
// C'est le cas d'école du 17/11 chez la voisine : un mauvais preset sélectionné,
// le moteur refuse de démarrer, et c'est exactement le moment où il faut pouvoir
// choisir l'autre. Si le pilotage exigeait un modèle en vie, la panne serait
// incurable à distance — il faudrait remettre la main dessus physiquement.
//
// Le test ne SIMULE PAS l'absence de moteur : il ne lance aucun moteur, et
// vérifie que les trois routes répondent quand même. startParallelBridge ne
// regarde que le pont demandé et la clé API (parallel_bridge.go:486-501), jamais
// un moteur — le test échouerait si quelqu'un y ajoutait une attente de moteur.
func TestPilotageSansModeleCharge(t *testing.T) {
	home := parallelTestHome(t)
	// La clé que le pont exige, posée comme les autres tests du pont le font
	// (parallel_bridge_test.go:58). Un test qui se dérobait faute de clé n'aurait
	// rien prouvé du tout : il aurait « réussi » sans jamais jouer l'exigence.
	if err := writeAPIKey("sk-pilotage-sans-modele"); err != nil {
		t.Fatal(err)
	}
	if err := setParallelBridgeControlOn(true); err != nil {
		t.Fatal(err)
	}

	// Aucun moteur n'est lancé, exprès. Ce qu'on vérifie est précisément l'absence
	// de ce réflexe : une route de réglage ne doit pas dépendre d'un cerveau.
	h := parallelBridgeHandler()

	appeler := func(methode, chemin, corps string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		var corpsLecteur *strings.Reader = strings.NewReader(corps)
		req := httptest.NewRequest(methode, "http://127.0.0.1"+chemin, corpsLecteur)
		if corps != "" {
			req.Header.Set("Content-Type", "application/json")
		}
		req.Header.Set(canalHeader, "1")
		req.Header.Set(canalFromHeader, "Cresius")
		req.Header.Set(canalOriginHeader, canalOrigineHumain)
		req.Header.Set("Authorization", "Bearer "+machineAPIKeyForTest(t))
		h.ServeHTTP(w, req)
		return w
	}

	// La clé doit être posée : le pont l'exige pour démarrer. On la lit là où le
	// pont la lit, pas où je crois qu'elle est.
	if readAPIKey() == "" {
		t.Skip("le pont exige une clé API machine ; base de test sans clé")
	}

	// 1) LIRE la liste des presets du poste, sans moteur.
	w := appeler(http.MethodGet, "/v1/presets", "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /v1/presets SANS moteur chargé : %d\n%s\nLe pilotage dépend d'un modèle en vie : la panne serait incurable à distance.",
			w.Code, w.Body.String())
	}
	var liste struct {
		OK      bool `json:"ok"`
		Presets []struct {
			ID     string `json:"id"`
			Active bool   `json:"active"`
		} `json:"presets"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &liste); err != nil {
		t.Fatal(err)
	}
	if !liste.OK {
		t.Fatalf("liste rendue sans ok:true : %s", w.Body.String())
	}

	// 2) Un preset posé sur disque, comme chez elle : on vérifie qu'on peut en
	// ACTIVER un alors que rien ne tourne.
	if err := os.MkdirAll(presetsDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(presetsDir(), "Pilotage-Sans-Moteur.env"),
		// MOE_RUNTIME=strata-windows est ce qui fait un moteur Strata portable :
		// sans lui, isStrataMoe refuse le preset (« les anciens moteurs MoE système
		// restent désactivés », parallel_config.go:110) et le basculement répond 404
		// pour une raison qui n'a rien à voir avec ce qu'on teste.
		// MOE_BACKEND=hip vient de la même garde (L135) : hip ou cuda, pas absent.
		// C'est ce que porte le preset de la voisine.
		[]byte("# NAME=Pilotage Sans Moteur\nENGINE=moe\nMOE_RUNTIME=strata-windows\nMOE_BACKEND=hip\nMOE_CONFIG="+home+"\\moe\\strata-0.1.57-hip\\serve\\server.py\nMOE_SOURCE="+home+"\\moe\\strata-0.1.57-hip\\serve\\server.py\nMOE_PYTHON="+home+"\\moe\\strata-0.1.57-hip\\python\\python.exe\nPORT=18080\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	w = appeler(http.MethodGet, "/v1/presets", "")
	if err := json.Unmarshal(w.Body.Bytes(), &liste); err != nil {
		t.Fatal(err)
	}
	idVise := ""
	for _, p := range liste.Presets {
		if strings.Contains(p.ID, "Pilotage-Sans-Moteur") {
			idVise = p.ID
		}
	}
	if idVise == "" {
		t.Fatalf("preset posé sur disque et absent de la liste rendue sans moteur : %s", w.Body.String())
	}

	// 3) LE BASCULEMENT, sans moteur. C'est l'acte. Il doit être accepté : le
	// redémarrage du service part en arrière-plan (switchToPresetByPath rend la
	// main avant), donc la réponse revient même sans cerveau derrière.
	w = appeler(http.MethodPost, "/v1/presets/switch", `{"id":"`+idVise+`"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("POST /v1/presets/switch SANS moteur chargé : %d\n%s", w.Code, w.Body.String())
	}
	var bascule struct {
		OK     bool   `json:"ok"`
		Preset string `json:"preset"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &bascule); err != nil {
		t.Fatal(err)
	}
	if !bascule.OK || !strings.Contains(bascule.Preset, "Pilotage Sans Moteur") {
		t.Fatalf("bascule rendue sans le nom du preset visé : %s", w.Body.String())
	}

	// 4) L'agent, même absence de moteur : c'est un réglage, pas une génération.
	w = appeler(http.MethodPost, "/v1/mode", `{"agent":"on"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("POST /v1/mode SANS moteur chargé : %d\n%s", w.Code, w.Body.String())
	}
}

// machineAPIKeyForTest : la clé que le pont exige, lue là où il la lit. Le test
// l'a posée via writeAPIKey ; la relire évite de coder en dur une valeur que
// writeAPIKey pourrait transformer (empaquement, préfixe ajouté).
func machineAPIKeyForTest(t *testing.T) string {
	t.Helper()
	k := readAPIKey()
	if k == "" {
		t.Fatal("writeAPIKey n'a rien écrit : la clé que le pont exige est introuvable")
	}
	return k
}

// TestRelaisPilotageVoisin : le bout en bout de la demande « piloter un AUTRE
// Ajean depuis le menu ». Navigateur → /api/peers/pilot → relais Go → pont du
// voisin. Le voisin est un vrai serveur HTTP, pas un bouchon à appeler en direct :
// c'est le trajet qui m'intéresse, pas la fonction.
func TestRelaisPilotageVoisin(t *testing.T) {
	parallelTestHome(t)

	var recus enTetesVoisin
	voisin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recus.noter(r)
		switch {
		case r.URL.Path == "/v1/presets" && r.Method == http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"presets":[{"id":"gyro-57","name":"GYRO-S","active":true,"moteur":"0.1.57"}],"moteur_epingle":"0.1.57"}`))
		case r.URL.Path == "/v1/presets/switch" && r.Method == http.MethodPost:
			var corps map[string]string
			_ = json.NewDecoder(r.Body).Decode(&corps)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"preset":"GYRO-S","id_recu":"` + corps["id"] + `"}`))
		case r.URL.Path == "/v1/mode" && r.Method == http.MethodPost:
			var corps map[string]string
			_ = json.NewDecoder(r.Body).Decode(&corps)
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"agent":` + map[bool]string{true: "true", false: "false"}[corps["agent"] == "on"] + `}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer voisin.Close()

	if _, err := peerAddDeclared("Voisin-de-test", voisin.URL, "cle-voisine", ""); err != nil {
		t.Fatal(err)
	}

	piloter := func(corps string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		handlePeerPilot(w, httptest.NewRequest(http.MethodPost, "/api/peers/pilot", strings.NewReader(corps)))
		return w
	}

	// 1) LIRE les presets du voisin, et le millésime avec. C'est LA pièce qui
	// manquait le 17/11 : l'interface n'affichait que le # NAME du fichier.
	w := piloter(`{"name":"Voisin-de-test","action":"presets"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("action presets : %d\n%s", w.Code, w.Body.String())
	}
	var liste struct {
		OK      bool `json:"ok"`
		Presets []struct {
			ID     string `json:"id"`
			Moteur string `json:"moteur"`
		} `json:"presets"`
		MoteurEpingle string `json:"moteur_epingle"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &liste); err != nil {
		t.Fatalf("la réponse du relais n'est pas le JSON du voisin : %v\n%s", err, w.Body.String())
	}
	if !liste.OK || len(liste.Presets) != 1 || liste.Presets[0].Moteur != "0.1.57" {
		t.Fatalf("millésime du voisin perdu en route : %s", w.Body.String())
	}
	if liste.MoteurEpingle != "0.1.57" {
		t.Fatalf("le moteur épinglé du voisin n'est pas rendu : %s", w.Body.String())
	}

	// 2) LES EN-TETES DU CANAL doivent arriver chez le voisin. Sans eux, son pont
	// voit du logiciel tiers : la route répond 404 ou 403, et le pilote ne sait pas
	// qui commande. C'est le contrat, pas un détail d'implémentation.
	if !recus.canal {
		t.Error("X-Aidros-Canal absent chez le voisin : son pont traiterait ça comme du logiciel tiers")
	}
	if recus.de == "" {
		t.Error("X-Aidros-Canal-From absent : le voisin ne sait pas quelle machine commande")
	}
	if recus.origine != canalOrigineHumain {
		t.Errorf("X-Aidros-Canal-Origin = %q, attendu %q : depuis l'interface c'est la personne qui a cliqué, et elle ne doit pas se faire passer pour une IA",
			recus.origine, canalOrigineHumain)
	}
	if recus.cle != "Bearer cle-voisine" {
		t.Errorf("la clé du voisin n'est pas partie chez lui : %q", recus.cle)
	}

	// 3) LA BASCULE, avec l'id rendu à l'étape 1 — jamais une position.
	w = piloter(`{"name":"Voisin-de-test","action":"switch","id":"gyro-57"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("action switch : %d\n%s", w.Code, w.Body.String())
	}
	var bascule struct {
		OK     bool   `json:"ok"`
		IDRecu string `json:"id_recu"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &bascule); err != nil {
		t.Fatal(err)
	}
	if !bascule.OK || bascule.IDRecu != "gyro-57" {
		t.Fatalf("l'id du preset n'est pas arrivé tel quel chez le voisin : %s", w.Body.String())
	}

	// 4) L'AGENT du voisin.
	w = piloter(`{"name":"Voisin-de-test","action":"agent","on":true}`)
	if w.Code != http.StatusOK {
		t.Fatalf("action agent : %d\n%s", w.Code, w.Body.String())
	}
	var agent struct {
		OK    bool `json:"ok"`
		Agent bool `json:"agent"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &agent); err != nil {
		t.Fatal(err)
	}
	if !agent.OK || !agent.Agent {
		t.Fatalf("l'agent du voisin n'est pas allumé : %s", w.Body.String())
	}

	// 5) LA CLÉ DU VOISIN NE TRAVERSE PAS LE NAVIGATEUR. GET /api/peers ne rend
	// que has_key : si quelqu'un ajoute la clé au JSON, elle sort du magasin local
	// et se retrouve dans le DOM. Le filet mord.
	w = httptest.NewRecorder()
	handleParallelPeers(w, httptest.NewRequest(http.MethodGet, "/api/peers", nil))
	if strings.Contains(w.Body.String(), "cle-voisine") {
		t.Fatalf("la CLÉ du voisin fuit dans GET /api/peers, donc dans le navigateur : %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "has_key") {
		t.Fatalf("GET /api/peers ne rend plus has_key : %s", w.Body.String())
	}
}

// TestRelaisPilotageAuteur : QUI a donné l'ordre, selon qui l'a déclenché.
//
// Depuis l'interface, c'est la personne. Si c'est un agent local qui passe par la
// même route (AJEAN_AGENT_TURN posé, comme chat_tools.go:235 le fait pour ce qu'il
// lance), le relais doit dire « agent ». M'attribuer l'autorité humaine serait la
// faute que l'arbitrage du 27/10 interdit.
func TestRelaisPilotageAuteur(t *testing.T) {
	parallelTestHome(t)

	var recus enTetesVoisin
	voisin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recus.noter(r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"agent":true}`))
	}))
	defer voisin.Close()

	if _, err := peerAddDeclared("Voisin-d-auteur", voisin.URL, "cle", ""); err != nil {
		t.Fatal(err)
	}

	// L'humain, devant son écran.
	t.Setenv("AJEAN_AGENT_TURN", "")
	w := httptest.NewRecorder()
	handlePeerPilot(w, httptest.NewRequest(http.MethodPost, "/api/peers/pilot",
		strings.NewReader(`{"name":"Voisin-d-auteur","action":"agent","on":true}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("pilotage par l'humain : %d\n%s", w.Code, w.Body.String())
	}
	if recus.origine != canalOrigineHumain {
		t.Errorf("clic de l'interface rendu %q, attendu %q", recus.origine, canalOrigineHumain)
	}

	// ET MÊME si le process de l'application a été lancé pendant un tour d'agent.
	// C'est le défaut que ce test a attrapé le 19/11 : AJEAN_AGENT_TURN est posé
	// dans l'environnement (chat_tools.go:235 le transmet à tout ce qu'il lance),
	// donc peerOrigineCLI() y lisait « agent » et un simple CLIC de la personne
	// était rendu comme une initiative d'IA — l'ordre humain perdait son autorité
	// chez le voisin, qui pouvait le traiter comme ne le regardant pas.
	//
	// La route est humaine par construction : un agent local ne passe pas par
	// HTTP pour piloter un voisin, il a l'outil peer (peerAsk). Le test vérifie
	// les DEUX côtés du contrat, pour que la correction ne soit pas un pansement.
	recus = enTetesVoisin{}
	t.Setenv("AJEAN_AGENT_TURN", "1")
	w = httptest.NewRecorder()
	handlePeerPilot(w, httptest.NewRequest(http.MethodPost, "/api/peers/pilot",
		strings.NewReader(`{"name":"Voisin-d-auteur","action":"agent","on":true}`)))
	if w.Code != http.StatusOK {
		t.Fatalf("pilotage avec AJEAN_AGENT_TURN posé : %d\n%s", w.Code, w.Body.String())
	}
	if recus.origine != canalOrigineHumain {
		t.Errorf("clic de l'interface rendu %q alors que le process tourne en tour d'agent, attendu %q : la personne devant l'écran commande, l'environnement du process ne décide pas de qui a cliqué",
			recus.origine, canalOrigineHumain)
	}

	// L'autre côté : le chemin de l'OUTIL peer, lui, doit bien dire « agent »
	// pendant un tour. Sinon ma correction aurait déplacé le mensonge au lieu de
	// le supprimer : une IA qui pilote par son outil passerait pour un humain.
	if got := peerOrigineCLI(); got != canalOrigineIA {
		t.Errorf("peerOrigineCLI() = %q avec AJEAN_AGENT_TURN posé, attendu %q : le chemin de l'outil doit rendre la vérité", got, canalOrigineIA)
	}
	t.Setenv("AJEAN_AGENT_TURN", "")
	if got := peerOrigineCLI(); got != canalOrigineHumain {
		t.Errorf("peerOrigineCLI() = %q hors tour d'agent, attendu %q : la commande tapée par l'humain dans son terminal est un ordre humain", got, canalOrigineHumain)
	}
}

// TestRelaisPilotageErreurs : les refus, joués. Un relais qui rend 200 sur un
// voisin injoignable serait pire que rien : la carte afficherait un succès.
func TestRelaisPilotageErreurs(t *testing.T) {
	parallelTestHome(t)

	// Voisin inconnu : 404, et le message dit quoi faire.
	w := httptest.NewRecorder()
	handlePeerPilot(w, httptest.NewRequest(http.MethodPost, "/api/peers/pilot",
		strings.NewReader(`{"name":"fantôme","action":"presets"}`)))
	if w.Code != http.StatusNotFound {
		t.Fatalf("voisin inconnu : %d, attendu 404\n%s", w.Code, w.Body.String())
	}

	// Action inconnue : 400, jamais un 200 qui ne fait rien.
	if _, err := peerAddDeclared("Voisin-connu", "http://127.0.0.1:1", "cle", ""); err != nil {
		t.Fatal(err)
	}
	w = httptest.NewRecorder()
	handlePeerPilot(w, httptest.NewRequest(http.MethodPost, "/api/peers/pilot",
		strings.NewReader(`{"name":"Voisin-connu","action":"coucou"}`)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("action inconnue : %d, attendu 400\n%s", w.Code, w.Body.String())
	}

	// Switch sans id : 400. L'id est ce que le voisin comprend, pas une position.
	w = httptest.NewRecorder()
	handlePeerPilot(w, httptest.NewRequest(http.MethodPost, "/api/peers/pilot",
		strings.NewReader(`{"name":"Voisin-connu","action":"switch"}`)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("switch sans id : %d, attendu 400\n%s", w.Code, w.Body.String())
	}

	// Agent sans le champ on : 400. « Pas demandé » et « faux » sont deux choses.
	w = httptest.NewRecorder()
	handlePeerPilot(w, httptest.NewRequest(http.MethodPost, "/api/peers/pilot",
		strings.NewReader(`{"name":"Voisin-connu","action":"agent"}`)))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("agent sans on : %d, attendu 400\n%s", w.Code, w.Body.String())
	}

	// Voisin déclaré mais injoignable : le relais doit SONNER l'échec, pas le
	// masquer. C'est le 502 du pont voisin, rendu en erreur lisible.
	w = httptest.NewRecorder()
	handlePeerPilot(w, httptest.NewRequest(http.MethodPost, "/api/peers/pilot",
		strings.NewReader(`{"name":"Voisin-connu","action":"presets"}`)))
	if w.Code == http.StatusOK {
		t.Fatalf("voisin injoignable rendu 200 : la carte afficherait un succès pour rien\n%s", w.Body.String())
	}
}

// enTetesVoisin : ce que le faux voisin a reçu. Le contrat du canal se joue là,
// pas dans ce que mon code croit envoyer.
//
// Un seul casier suffit, et le test du 19/11 en a vérifié la raison : les TROIS
// actions du relais (lire les presets, basculer, allumer l'agent) partent d'un
// CLIC de la personne, donc portent toutes la même origine. Ce n'est pas le
// chemin de l'outil peer, où c'est l'IA qui parle — celui-là est couvert par
// TestRelaisPilotageAuteur, qui joue les deux côtés de la frontière.
//
// ⚠ Le test écrase ce casier à chaque requête : il ne faut lire un champ qu'après
// L'action qui l'intéresse, jamais « à la fin ». C'est ce qui m'a fait rendre un
// échec pour la mauvaise raison à la première version.
type enTetesVoisin struct {
	canal   bool
	de      string
	origine string
	cle     string
}

func (e *enTetesVoisin) noter(r *http.Request) {
	e.canal = r.Header.Get(canalHeader) == "1"
	e.de = r.Header.Get(canalFromHeader)
	e.origine = r.Header.Get(canalOriginHeader)
	e.cle = r.Header.Get("Authorization")
}
