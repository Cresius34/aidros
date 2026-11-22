package aidros

// Deux régressions trouvées en chassant les avertissements « symbole inutilisé »
// de staticcheck (22/11). Leçon : un symbole mort est soit du code mort, soit un
// routage perdu ; la seconde réponse coûte une fonctionnalité, pas un bruit.
//
//  1. web_api.go:557-558 portait DEUX sendJSON consécutifs : /api/preset rendait
//     deux objets JSON collés. Blâme : e9ece17 (moi, 15/11) a rétabli le champ
//     « sysprompt » SANS retirer l'envoi d'en dessous, posé par 4ecd150 (14/11).
//     Conséquence mesurée : un client strict rejette la réponse — json.loads côté
//     Python échoue sur « Extra data » (tools/embedding-test/smoke_portable.py:128
//     lit cette route), et response.json() côté navigateur aussi.
//
//  2. handleReasoning n'appelait plus moePersistEffort. En v0.18.17, ce même
//     handler l'appelait juste après SetConfigKey (web_api.go:340, mesuré). La
//     campagne de rétablis du 15/11 (93c24d0) a rendu les autres ponts sans
//     celui-ci. Conséquence pour l'utilisateur : REASONING_EFFORT n'est PAS dans
//     preservedKeys (backend_presets.go:281-284), donc le niveau de réflexion
//     choisi dans le composeur repartait au défaut à chaque bascule de preset MoE.
//
// Les deux filets sont écrits AVANT de vérifier qu'ils vertissent, et la
// contre-épreuve (rouge sans le correctif) est jouée après.

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// /api/preset doit rendre un seul objet JSON, et le champ « sysprompt » notre
// extension doit y figurer. Le test ne se contente pas de décoder : il exige
// que le reste du corps soit vide, ce que le second sendJSON violait.
func TestPresetCarteRepondUnSeulObjetJSON(t *testing.T) {
	testHome(t)

	id, err := SavePreset("", "CarteJSON", "MODEL=a.gguf\nCTX=4096\n")
	if err != nil {
		t.Fatal(err)
	}
	// Notre extension : un prompt système porté par le preset, stocké en base.
	if err := setPresetSysPrompt(id, "Tu es vif."); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	handlePreset(w, httptest.NewRequest("GET", "/api/preset?id="+id, nil))

	corps := w.Body.String()

	// 1. Un seul objet : le décodeur lit le premier, puis doit tomber sur la fin.
	dec := json.NewDecoder(w.Body)
	var carte map[string]any
	if err := dec.Decode(&carte); err != nil {
		t.Fatalf("/api/preset ne rend pas un objet JSON : %v\n%s", err, corps)
	}
	if err := dec.Decode(&map[string]any{}); err != io.EOF {
		t.Errorf("/api/preset rend PLUSIEURS objets JSON collés (le second envoyait la réponse sans « sysprompt ») : err = %v\n%s", err, corps)
	}

	// 2. Le pont rétabli le 15/11 est bien là.
	if carte["sysprompt"] != "Tu es vif." {
		t.Errorf("champ sysprompt = %v, attendu « Tu es vif. » — le pont preset→carte est rompu. Clés rendues : %v", carte["sysprompt"], cles(carte))
	}
	if carte["id"] != id {
		t.Errorf("champ id = %v, attendu %q", carte["id"], id)
	}

	// 3. La clé ne doit pas couler sur un preset qui ne porte pas de prompt.
	idVide, err := SavePreset("", "SansPrompt", "MODEL=b.gguf\nCTX=2048\n")
	if err != nil {
		t.Fatal(err)
	}
	w2 := httptest.NewRecorder()
	handlePreset(w2, httptest.NewRequest("GET", "/api/preset?id="+idVide, nil))
	var carte2 map[string]any
	if err := json.Unmarshal(w2.Body.Bytes(), &carte2); err != nil {
		t.Fatalf("/api/preset (second preset) ne rend pas un objet JSON : %v\n%s", err, w2.Body.String())
	}
	if carte2["sysprompt"] != "" {
		t.Errorf("preset sans prompt système : sysprompt = %v, attendu chaîne vide", carte2["sysprompt"])
	}
}

// Le raccourci « niveau de réflexion » du composeur doit inscrire l'effort dans
// le fichier du preset MoE actif, pas seulement dans la config vivante : sinon
// la bascule suivante efface le choix (REASONING_EFFORT n'est pas préservée).
func TestEffortInscritDansLePresetActif(t *testing.T) {
	testHome(t)

	id, err := SavePreset("", "MoEEffort", "ENGINE=moe\nMOE_MODEL=a.gguf\nCTX=65536\n")
	if err != nil {
		t.Fatal(err)
	}
	// Le preset doit être le preset ACTIF au sens large : moePersistEffort refuse
	// si la config VIVANTE n'est pas un MoE (isMoeConfig(ReadConfig())), puis
	// lit l'identifiant en base. applyPresetFile pose les deux d'un coup (il
	// écrit la config et le marqueur « active_preset », backend_presets.go:355) ;
	// putStr seul ne remplirait que la seconde condition.
	p, err := safePresetPath(id)
	if err != nil {
		t.Fatal(err)
	}
	if err := applyPresetFile(p); err != nil {
		t.Fatal(err)
	}
	if !isMoeConfig(ReadConfig()) {
		t.Fatal("applyPresetFile n'a pas rendu la config vivante MoE : le test ne prouverait rien")
	}

	env := func() map[string]string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(presetsDir(), id+".env"))
		if err != nil {
			t.Fatalf("fichier preset illisible : %v", err)
		}
		return parseEnv(string(b))
	}
	if _, lu := env()["REASONING_EFFORT"]; lu {
		t.Fatal("le preset porte déjà REASONING_EFFORT : le test ne prouverait plus rien")
	}

	body := []byte(`{"effort":"high"}`)
	w := httptest.NewRecorder()
	handleReasoning(w, httptest.NewRequest("POST", "/api/reasoning", bytes.NewReader(body)))
	if w.Code != 200 {
		t.Fatalf("/api/reasoning a répondu %d : %s", w.Code, w.Body.String())
	}

	if got := env()["REASONING_EFFORT"]; got != "high" {
		t.Errorf("REASONING_EFFORT dans le fichier preset = %q, attendu « high » — le niveau repartira au défaut à la bascule. Contenu lu : %v", got, env())
	}
	// Le fichier doit rester un preset MoE valide : pas de duplication, pas de casse.
	if got := env()["ENGINE"]; got != "moe" {
		t.Errorf("ENGINE = %q après l'écriture de l'effort", got)
	}

	// Un preset NON actif ne doit pas être touché : l'effort choisi concerne le
	// preset qui tourne, pas le premier trouvé sur le disque.
	idTiers, err := SavePreset("", "MoETiers", "ENGINE=moe\nMOE_MODEL=b.gguf\n")
	if err != nil {
		t.Fatal(err)
	}
	if err := putStr(bkState, "active_preset", ""); err != nil {
		t.Fatal(err)
	}
	w2 := httptest.NewRecorder()
	handleReasoning(w2, httptest.NewRequest("POST", "/api/reasoning", bytes.NewReader([]byte(`{"effort":"low"}`))))
	if _, lu := envOf(t, idTiers)["REASONING_EFFORT"]; lu {
		t.Error("aucun preset n'est actif et pourtant un preset a reçu REASONING_EFFORT")
	}
}

func envOf(t *testing.T, id string) map[string]string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(presetsDir(), id+".env"))
	if err != nil {
		t.Fatalf("fichier preset %q illisible : %v", id, err)
	}
	return parseEnv(string(b))
}
