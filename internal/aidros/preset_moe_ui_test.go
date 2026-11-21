package aidros

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

// L'engrenage des reglages d'un preset depend d'une seule cle : x.moe.
// Cote interface (06-settings.js:143), l'engrenage appelle
//
//	x.moe ? openMoe(x.id) : openPreset(x.id)
//
// Sans la cle, un preset MoE retombe donc sur l'editeur llama.cpp : la fenetre
// Strata ne s'ouvre jamais. Or la fusion amont 0.20.1 a supprime
// backend_exl3.go / backend_cloud.go / voice.go et a tronque web_api.go ; la
// branche isMoeConfig de handlePresets a disparu avec eux, sans qu'aucun test
// ne s'en plaigne. Ce test est la pour ne plus jamais repartir a l'aveugle.
func TestPresetsMoERendentLaCleMoe(t *testing.T) {
	testHome(t)

	// Un preset MoE, tel que ceux de l'atelier : ENGINE=moe, vision activee.
	idMoe, err := SavePreset("", "MoE Test", "ENGINE=moe\nMOE_MODEL=a.gguf\nMOE_VISION=1\n")
	if err != nil {
		t.Fatal(err)
	}
	// Un preset llama.cpp, pour prouver que la cle ne coule pas sur les autres.
	idLlama, err := SavePreset("", "Llama Test", "MODEL=b.gguf\nCTX=4096\n")
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	handlePresets(w, httptest.NewRequest("GET", "/api/presets", nil))

	// handlePresets rend le tableau TEL QUEL (sendJSON(w, 200, out)), sans
	// enveloppe « {"presets":…} » : le décoder comme un objet laisserait la
	// liste vide et le test passerait sans rien vérifier.
	var out []map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("/api/presets ne rend pas un tableau JSON : %v\n%s", err, w.Body.String())
	}

	byID := map[string]map[string]any{}
	for _, it := range out {
		if id, ok := it["id"].(string); ok {
			byID[id] = it
		}
	}
	t.Logf("/api/presets rend %d preset(s)", len(out))

	it, ok := byID[idMoe]
	if !ok {
		t.Fatalf("preset MoE absent de /api/presets (id %q) — rendus : %v", idMoe, cles(byID))
	}
	if it["moe"] != true {
		t.Fatalf("preset MoE : cle moe = %v, attendu true — l'interface ouvrira l'editeur llama.cpp a la place du configurateur Strata. Cles rendues : %v",
			it["moe"], cles(it))
	}
	if it["vision"] != true {
		t.Errorf("preset MoE avec MOE_VISION=1 : cle vision = %v, attendu true", it["vision"])
	}

	ll, ok := byID[idLlama]
	if !ok {
		t.Fatalf("preset llama.cpp absent de /api/presets (id %q)", idLlama)
	}
	if _, existe := ll["moe"]; existe {
		t.Errorf("preset llama.cpp : la cle moe ne doit pas etre rendue, recu %v", ll["moe"])
	}
}

// Les touches d'une carte, dans le désordre : de quoi lire dans un message
// d'echec ce que /api/presets a vraiment rendu.
func cles[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
