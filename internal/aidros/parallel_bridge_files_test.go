package aidros

// Le filet de l'échange de fichiers par le pont.
//
// Il appelle le HANDLER DU PONT, pas handleEchange directement. Raison, mesurée le 15/11 et écrite
// dans preflight_moe_cresius_test.go:55-62 : quand l'appel à serveMoe avait disparu de l'arbre,
// le test du pré-vol restait VERT parce qu'il n'appelait pas serve(). Tester la garde sans tester
// l'aiguillage laisse partir une route orpheline — et ici, deux placements fautifs de la route
// (trop bas : 404 par la garde /v1/* de parallel_bridge.go:168 ; après canalDemande : avalé par un
// tour d'agent) sont tous les deux SILENCIEUX.
//
// Il vérifie aussi la DONNÉE et pas seulement le code : la leçon de preset_moe_ui_test.go:36 est
// qu'un tableau rendu TEL QUEL, décoder comme une enveloppe laisse la liste VIDE et le test passe
// sans rien contrôler.

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func echangeTestCle(t *testing.T) {
	t.Helper()
	if err := writeAPIKey("sk-echange-test"); err != nil {
		t.Fatal(err)
	}
}

func echangeTestRequete(methode, chemin, corps string, entetes map[string]string) *http.Request {
	var corpsLecteur *bytes.Reader = bytes.NewReader(nil)
	if corps != "" {
		corpsLecteur = bytes.NewReader([]byte(corps))
	}
	req := httptest.NewRequest(methode, "http://192.168.1.9"+chemin, corpsLecteur)
	// Depuis le réseau, pas la boucle locale : sinon antKeyOK admet sans clé (anthropic_api.go:302)
	// et le test ne prouverait plus que la route est protégée.
	req.RemoteAddr = "192.168.1.42:51515"
	req.Header.Set("Authorization", "Bearer sk-echange-test")
	for k, v := range entetes {
		req.Header.Set(k, v)
	}
	return req
}

func TestEchangeFichiersPoseListeRelit(t *testing.T) {
	parallelTestHome(t)
	echangeTestCle(t)
	h := parallelBridgeHandler()

	// 1) liste à vide : le dossier n'existe pas encore, la route doit répondre QUAND MÊME.
	w := httptest.NewRecorder()
	h.ServeHTTP(w, echangeTestRequete("GET", "/v1/files", "", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("liste initiale : %d\n%s", w.Code, w.Body.String())
	}
	fichiers, _, err := echangeDecodageListe(w.Body.Bytes())
	if err != nil {
		t.Fatalf("liste initiale indécodable : %v\n%s", err, w.Body.String())
	}
	if len(fichiers) != 0 {
		t.Fatalf("dossier d'échange peuplé avant d'avoir rien posé : %v", fichiers)
	}

	// 2) pose. Contenu choisi pour ne PAS être une longueur de bloc : un troncage changerait la
	// taille visible, un contenu calibré la masquerait.
	contenu := "la piece qui manquait, 196 632 octets chez la voisine, et ici trois cent onze\n"
	w = httptest.NewRecorder()
	h.ServeHTTP(w, echangeTestRequete("POST", "/v1/files/profil-test.bin", contenu, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("pose : %d\n%s", w.Code, w.Body.String())
	}
	var pose struct {
		OK           bool   `json:"ok"`
		Nom          string `json:"nom"`
		TailleOctets int64  `json:"taille_octets"`
		Sha256       string `json:"sha256"`
		Error        string `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &pose); err != nil {
		t.Fatalf("pose indécodable : %v\n%s", err, w.Body.String())
	}
	somme := sha256.Sum256([]byte(contenu))
	attendu := hex.EncodeToString(somme[:])
	if !pose.OK || pose.Nom != "profil-test.bin" || pose.TailleOctets != int64(len(contenu)) || pose.Sha256 != attendu {
		t.Fatalf("pose mal rendue : %+v (empreinte attendue %s)", pose, attendu)
	}
	// Le fichier est bien sur le disque, et à l'ENDROIT prévu.
	surDisque := filepath.Join(echangeDir(), "profil-test.bin")
	if _, err := os.Stat(surDisque); err != nil {
		t.Fatalf("posé mais pas sur le disque (%s) : %v", surDisque, err)
	}
	// Pas de .part oublié derrière nous.
	if reste, err := os.ReadDir(echangeDir()); err == nil {
		for _, e := range reste {
			if strings.HasSuffix(e.Name(), ".part") {
				t.Errorf("un .part est resté : %s", e.Name())
			}
		}
	}

	// 3) relecture par la route : les OCTETS, et l'empreinte dans l'en-tête.
	w = httptest.NewRecorder()
	h.ServeHTTP(w, echangeTestRequete("GET", "/v1/files/profil-test.bin", "", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("relecture : %d\n%s", w.Code, w.Body.String())
	}
	if w.Body.String() != contenu {
		t.Errorf("octets relus différents de ce qui a été posé\nobtenu %q", w.Body.String())
	}
	if w.Header().Get(echangeEnTeteSha) != attendu {
		t.Errorf("empreinte de relecture %q, attendu %s", w.Header().Get(echangeEnTeteSha), attendu)
	}

	// 4) la liste rend bien la DONNÉE, pas un tableau vide lu comme une enveloppe.
	w = httptest.NewRecorder()
	h.ServeHTTP(w, echangeTestRequete("GET", "/v1/files", "", nil))
	fichiers, total, err := echangeDecodageListe(w.Body.Bytes())
	if err != nil {
		t.Fatalf("liste : %v\n%s", err, w.Body.String())
	}
	if len(fichiers) != 1 || total != int64(len(contenu)) {
		t.Fatalf("liste fausse : %d entrée(s) total %d\n%s", len(fichiers), total, w.Body.String())
	}
	if fichiers[0]["nom"] != "profil-test.bin" || fichiers[0]["sha256"] != attendu {
		t.Errorf("entrée de liste : %v", fichiers[0])
	}
}

func TestEchangeFichiersRefuses(t *testing.T) {
	parallelTestHome(t)
	echangeTestCle(t)
	h := parallelBridgeHandler()

	// Le nom vient du réseau. Chacun de ces noms doit être REFUSÉ, et refusé PAR NOUS (400), pas
	// traité. bbolt verrouille ajean.db en EXCLUSIF (store.go:46-53) : laisser « ..\..\ajean.db »
	// passer, c'est prêter la base active à un voisin par le réseau.
	for _, nom := range []string{
		`..\..\ajean.db`,
		`..\\..\\ajean.db`,
		`C:\Windows\win.ini`,
		`/etc/passwd`,
		`..`,
		`.cache`,
		`dos\ier\nom.bin`,
		`sous/nom.bin`,
	} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, echangeTestRequete("POST", "/v1/files/"+nom, "x", nil))
		if w.Code != http.StatusBadRequest {
			t.Errorf("nom %q : %d, attendu 400\n%s", nom, w.Code, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "clé API invalide") {
			t.Errorf("nom %q : pris pour un problème de clé", nom)
		}
	}
	// Rien n'a été écrit au passage.
	if entrees, err := os.ReadDir(echangeDir()); err == nil {
		for _, e := range entrees {
			t.Errorf("un refus a laissé un fichier : %s", e.Name())
		}
	}

	// Écrasement : jamais en silence.
	w := httptest.NewRecorder()
	h.ServeHTTP(w, echangeTestRequete("POST", "/v1/files/deux.bin", "un contenu", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("première pose : %d\n%s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, echangeTestRequete("POST", "/v1/files/deux.bin", "un autre contenu", nil))
	if w.Code != http.StatusConflict {
		t.Fatalf("écrasement sans en-tête : %d, attendu 409\n%s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, echangeTestRequete("POST", "/v1/files/deux.bin", "un autre contenu",
		map[string]string{echangeEnTeteRemplacer: "1"}))
	if w.Code != http.StatusOK {
		t.Fatalf("écrasement avec en-tête : %d\n%s", w.Code, w.Body.String())
	}

	// Fichier absent : 404, pas 200 avec un corps vide.
	w = httptest.NewRecorder()
	h.ServeHTTP(w, echangeTestRequete("GET", "/v1/files/absent-depuis-toujours.bin", "", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("fichier absent : %d, attendu 404", w.Code)
	}
}

// TestEchangeFichiersProtegesParLaCle : la route ne peut pas devenir une porte ouverte à côté du
// pont. Sans clé, depuis le réseau, c'est 401 — le même contrat que /v1/models (test
// TestParallelBridgeSurfaceAndAuth:72-79), parce que la route est branchée APRÈS le contrôle.
func TestEchangeFichiersProtegesParLaCle(t *testing.T) {
	parallelTestHome(t)
	echangeTestCle(t)
	h := parallelBridgeHandler()

	for _, chemin := range []string{"/v1/files", "/v1/files/un.bin"} {
		req := httptest.NewRequest("GET", "http://192.168.1.9"+chemin, nil)
		req.URL.Path = chemin
		req.RemoteAddr = "192.168.1.42:51515"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s sans clé : %d, attendu 401\n%s", chemin, w.Code, w.Body.String())
		}

		// Et la route fichiers ne déborde pas sur les autres surfaces du pont : hors /v1/*, c'est
		// toujours le 404 de la garde (parallel_bridge.go:168).
		req = httptest.NewRequest("GET", "http://192.168.1.9/files", nil)
		req.RemoteAddr = "192.168.1.42:51515"
		req.Header.Set("Authorization", "Bearer sk-echange-test")
		w = httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("/files (hors /v1) : %d, attendu 404 — la route déborde", w.Code)
		}
	}
}

// TestEchangeRouteAiguilleeDepuisLePont : la route est bien atteignable PAR LE PONT, avec l'en-tête
// du canal présent. C'est le placement le plus fin : trop bas sous la garde /v1/* → 404 ; branchée
// APRÈS canalDemande → un tour d'agent. Les deux rendent du texte, seul le test le dit.
func TestEchangeRouteAiguilleeDepuisLePont(t *testing.T) {
	parallelTestHome(t)
	echangeTestCle(t)
	h := parallelBridgeHandler()

	req := echangeTestRequete("GET", "/v1/files", "", map[string]string{canalHeader: "1"})
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("liste avec l'en-tête du canal : %d, attendu 200 — la route estavalée par le canal ?\n%s", w.Code, w.Body.String())
	}
	if _, _, err := echangeDecodageListe(w.Body.Bytes()); err != nil {
		t.Fatalf("réponse de liste indécodable depuis le canal : %v\n%s", err, w.Body.String())
	}
	// Si la route était branchée APRÈS canalDemande, c'est un tour d'agent qui répondrait, et le
	// pont poserait son en-tête de refus d'agent. Un refus présent = placement fautif.
	if refus := w.Header().Get(canalAgentRefusHeader); refus != "" {
		t.Errorf("la route fichiers a reçu un refus d'agent : %s — elle est branchée après le canal", refus)
	}
}
