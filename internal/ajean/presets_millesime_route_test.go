package ajean

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// TestPresetsMoERendentLeMillesime : la COULEUR de l'humain du 21/11 (« millésime
// sur presets en vert si endpoint = moteur installé sinon rouge ») ne sert à rien
// si elle ne traverse pas la route. millesime_avertissement_test.go prouve la
// fonction ; celui-ci prouve que /api/presets la rend, et qu'elle ne coule pas sur
// les presets qui n'ont pas de moteur MoE.
//
// Le précédent est réel et récent : la fusion amont 0.20.1 a supprimé la branche
// isMoeConfig de handlePresets « sans qu'aucun test ne s'en plaigne »
// (preset_moe_ui_test.go). Une cle rendue par une fonction mais pas par la route
// est une pastille qui ne s'affiche jamais.
func TestPresetsMoERendentLeMillesime(t *testing.T) {
	// UN SEUL home préparé, deux moteurs posés dedans. Appeler la fixture deux
	// fois aurait appelé testHome deux fois, donc changé AJEAN_HOME entre les
	// deux : la config du premier moteur, encodée relativement à l'ancien home,
	// ne se décode plus sous le nouveau et il passerait pour absent.
	parallelTestHome(t)
	vert := strataMoteurDansLeHome(t, strataVersion+"-hip", strataVersion)
	rouge := strataMoteurDansLeHome(t, "0.1.57-hip", "0.1.57")

	idVert, err := SavePreset("", "MoE Vert", strataPresetEnv(vert))
	if err != nil {
		t.Fatal(err)
	}
	idRouge, err := SavePreset("", "MoE Rouge", strataPresetEnv(rouge))
	if err != nil {
		t.Fatal(err)
	}
	idLlama, err := SavePreset("", "Llama Test", "MODEL=b.gguf\nCTX=4096\n")
	if err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	handlePresets(w, httptest.NewRequest("GET", "/api/presets", nil))

	// Le tableau TEL QUEL, sans enveloppe : le décoder comme un objet laisserait
	// la liste vide et le test passerait sans rien vérifier (piège déjà écrit
	// dans preset_moe_ui_test.go).
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

	it, ok := byID[idVert]
	if !ok {
		t.Fatalf("preset MoE vert absent de /api/presets (id %q)", idVert)
	}
	if it["moteur"] != strataVersion {
		t.Errorf("vert : cle moteur = %v, attendu %q. Cles rendues : %v", it["moteur"], strataVersion, cles(it))
	}
	if it["moteur_ok"] != true {
		t.Errorf("vert : moteur_ok = %v, attendu true — la pastille serait rouge. Avis rendu : %v", it["moteur_ok"], it["moteur_avis"])
	}
	if avis, existe := it["moteur_avis"]; existe {
		t.Errorf("vert : aucun avis attendu, recu %v", avis)
	}

	rd, ok := byID[idRouge]
	if !ok {
		t.Fatalf("preset MoE rouge absent de /api/presets (id %q)", idRouge)
	}
	if rd["moteur"] != "0.1.57" {
		t.Errorf("rouge : cle moteur = %v, attendu 0.1.57 (le moteur POSÉ, pas celui demandé)", rd["moteur"])
	}
	if rd["moteur_ok"] == true {
		t.Error("rouge : moteur_ok = true, la pastille serait verte")
	}
	avis, _ := rd["moteur_avis"].(string)
	if avis == "" {
		t.Fatal("rouge sans moteur_avis : la couleur ne dirait rien, et un refus qui ne nomme pas sa cause est le piège du 14/11")
	}
	// L'avis doit nommer les DEUX millésimes : celui que le preset veut dire et
	// celui qui est posé. C'est la confusion exacte qui a coûté quatre tours
	// chez la voisine le 17/11.
	for _, veut := range []string{strataVersion, "0.1.57"} {
		if !strings.Contains(avis, veut) {
			t.Errorf("rouge : l'avis doit nommer %q, rendu : %q", veut, avis)
		}
	}

	// Un preset sans moteur MoE ne doit pas porter de couleur : une pastille sur
	// un preset sans rapport fait mentir la légende.
	ll, ok := byID[idLlama]
	if !ok {
		t.Fatalf("preset llama.cpp absent de /api/presets (id %q)", idLlama)
	}
	for _, cle := range []string{"moteur", "moteur_ok", "moteur_avis"} {
		if _, existe := ll[cle]; existe {
			t.Errorf("preset llama.cpp : la cle %q ne doit pas etre rendue, recu %v", cle, ll[cle])
		}
	}
}

// strataPresetEnv : les cles telles qu'un fichier presets/*.env les porte, avec
// les chemins de la fixture. MOE_RUNTIME=strata-windows est ce qui fait un moteur
// Strata (isStrataMoe), et sans lui la question du millésime ne se pose pas.
func strataPresetEnv(c map[string]string) string {
	var b strings.Builder
	for _, k := range []string{"ENGINE", "MOE_RUNTIME", "MOE_BACKEND", "MOE_CONFIG", "MOE_SOURCE", "MOE_PYTHON", "MOE_VISION", "CTX", "MOE_KV"} {
		if v := c[k]; v != "" {
			b.WriteString(k + "=" + filepath.ToSlash(v) + "\n")
		}
	}
	return b.String()
}
