package ajean

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Le millésime du moteur ne REFUSE plus le lancement, il AVERTIT (ordre de
// l'humain du 21/11 : « prévoir avertissement + millésime sur presets en vert si
// endpoint = moteur installé sinon rouge »). Ces tests portent les trois états de
// la couleur, et le fait que le préflight passe.
//
// POURQUOI UN TEST SUR LE PRÉFLIGHT ET PAS SEULEMENT SUR LA COULEUR : baisser une
// garde sans la rendre lisible est pire qu'un refus. Un refus nommait le problème
// (« moteur MoE différent de la version figée ») ; un silence l'enterre. Donc la
// couleur est testée POUR CHAQUE cas où le préflight laisse passer.

// strataMoteurFixture pose un moteur MoE au sol, avec le millésime de SON DOSSIER
// et celui de SON BUILD.json choisis séparément. C'est la distinction qui a coûté
// quatre tours chez la voisine le 17/11 : le dossier dit ce que le PRESET veut
// dire, le BUILD.json dit ce qui EST LÀ.
//
// Lecture seule pour le reste : aucun processus, aucun modèle de 74 Go, aucun
// réseau (même discipline que home_preflight_cresius_test.go).
func strataMoteurFixture(t *testing.T, dossier, versionMoteur string) map[string]string {
	t.Helper()
	parallelTestHome(t)
	home := moeHome()
	src := filepath.Join(home, "strata-"+dossier)
	exe := filepath.Join(src, "engine", "strata.exe")
	py := filepath.Join(src, "python", "python.exe")
	cfg := filepath.Join(src, "strata-swift-iq3_xxs.json")
	for _, p := range []string{exe, py, filepath.Join(src, "serve", "server.py")} {
		if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("fixture, jamais execute"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	meta := map[string]any{"version": versionMoteur, "backend": "hip", "source": "release"}
	if err := strataWriteJSON(filepath.Join(filepath.Dir(exe), "BUILD.json"), meta); err != nil {
		t.Fatal(err)
	}
	base := map[string]any{"exe": exe, "args": []any{"--pack", filepath.Join(moeDataDir(), "packs", "swift-iq3_xxs"), "--max-context", "86016"}}
	encoded, err := strataPortableJSON(base, true)
	if err != nil {
		t.Fatal(err)
	}
	if err = strataWriteJSON(cfg, encoded); err != nil {
		t.Fatal(err)
	}
	return map[string]string{
		"ENGINE": "moe", "MOE_RUNTIME": "strata-windows", "MOE_BACKEND": "hip",
		"MOE_CONFIG": cfg, "MOE_SOURCE": src, "MOE_PYTHON": py,
		"CTX": "86016", "MOE_KV": "int8", "MOE_SPEC": "3", "MOE_VRAM_RESERVE": "4096", "MOE_VISION": "0",
	}
}

// TestPreflightAccepteMillésimeDivergent : LE CORRECTIF. Un moteur dont le
// BUILD.json ne porte pas le millésime épinglé doit DEMARRER QUAND MÊME. Sans le
// correctif, ce test est rouge sur le refus — c'est sa morsure.
func TestPreflightAccepteMillésimeDivergent(t *testing.T) {
	// Le cas Bekka, mesuré le 17/11 : son preset pointait strata-0.1.48-hip alors
	// que l'exe épinglait 0.1.57. Ici dans sa forme future : preset ET moteur
	// d'accord sur un millésime que CETTE application n'épingle pas.
	c := strataMoteurFixture(t, "0.1.57-hip", "0.1.57")
	if _, _, err := moeStrataPreflight(c); err != nil {
		t.Fatalf("le millésime refuse encore le lancement : %v", err)
	}
	// Et la couleur doit le dire, sinon la porte ouverte devient un trou muet.
	mv, ok, avis := strataMillésimePreset(c)
	if ok {
		t.Fatal("moteur 0.1.57 sur une application épinglée 0.1.59 : doit etre rouge")
	}
	if mv != "0.1.57" {
		t.Errorf("millésime rendu %q, attendu 0.1.57 (le nom du moteur posé, pas celui demande)", mv)
	}
	if !strings.Contains(avis, "0.1.57") || !strings.Contains(avis, strataVersion) {
		t.Errorf("l'avis doit nommer les DEUX millésimes, rendu : %q", avis)
	}
}

// TestMillésimePresetVert : le VERT de l'humain, « si endpoint = moteur installé ».
// Les trois millésimes d'accord : celui que le preset demande, celui que le moteur
// posé déclare, celui qu'a épinglé cette application.
func TestMillésimePresetVert(t *testing.T) {
	c := strataMoteurFixture(t, strataVersion+"-hip", strataVersion)
	if _, _, err := moeStrataPreflight(c); err != nil {
		t.Fatalf("meme millésime refuse : %v", err)
	}
	mv, ok, avis := strataMillésimePreset(c)
	if !ok {
		t.Fatalf("vert attendu, rendu rouge : %q", avis)
	}
	if mv != strataVersion {
		t.Errorf("millésime %q, attendu %q", mv, strataVersion)
	}
	if avis != "" {
		t.Errorf("aucun avis attendu sur un preset vert, rendu : %q", avis)
	}
}

// TestMillésimePresetRougePresetContreMoteur : le rouge qui a fait perdre quatre
// tours — le PRESET nomme un millésime que le dossier posé ne déclare pas.
func TestMillésimePresetRougePresetContreMoteur(t *testing.T) {
	c := strataMoteurFixture(t, strataVersion+"-hip", "0.1.48")
	mv, ok, avis := strataMillésimePreset(c)
	if ok {
		t.Fatal("preset 0.1.59 contre moteur pose 0.1.48 : doit etre rouge")
	}
	if mv != "0.1.48" {
		t.Errorf("millésime rendu %q, attendu 0.1.48 : la couleur doit dire ce qui EST LÀ", mv)
	}
	for _, veut := range []string{"0.1.59", "0.1.48"} {
		if !strings.Contains(avis, veut) {
			t.Errorf("l'avis doit nommer %q, rendu : %q", veut, avis)
		}
	}
}

// TestMillésimePresetMoteurAbsent : rouge, et l'avis doit nommer la pièce qui
// manque — le piège du 14/11 était un refus qui ne disait pas QUOI.
func TestMillésimePresetMoteurAbsent(t *testing.T) {
	c := strataMoteurFixture(t, strataVersion+"-hip", strataVersion)
	exe := filepath.Join(c["MOE_SOURCE"], "engine", "strata.exe")
	if err := os.Remove(exe); err != nil {
		t.Fatal(err)
	}
	if _, _, err := moeStrataPreflight(c); err == nil {
		t.Fatal("moteur absent accepte : la garde-la (piece manquante) ne joue plus")
	}
	_, ok, avis := strataMillésimePreset(c)
	if ok {
		t.Fatal("moteur absent : doit etre rouge")
	}
	if avis == "" {
		t.Fatal("rouge sans avis : la couleur ne dirait rien")
	}
}

// TestMillésimePresetNonMoE : un preset llama.cpp ou externe n'est NI vert NI
// rouge. Une couleur sur un preset sans rapport ferait mentir la légende.
func TestMillésimePresetNonMoE(t *testing.T) {
	c := map[string]string{"ENGINE": "llamacpp", "BIN": "x", "MODEL": "y"}
	if mv, _, avis := strataMillésimePreset(c); mv != "" || avis != "" {
		t.Errorf("preset non MoE juge : millésime %q avis %q", mv, avis)
	}
}
