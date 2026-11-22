package aidros

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMemoryGlobalModeAndPresetPriority(t *testing.T) {
	parallelTestHome(t)
	c := embeddingConfig{Enabled: false, Port: 18081, Threads: 8, Dimensions: 256}
	if err := saveEmbeddingConfig(c); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	handleEmbeddingMode(w, httptest.NewRequest("POST", "/api/embeddings/mode", strings.NewReader(`{"mode":"hybrid"}`)))
	if w.Code != 200 || !readEmbeddingDefaults().Enabled || !readEmbeddingConfig().Enabled {
		t.Fatal(w.Body.String())
	}
	if err := WriteConfig(map[string]string{"MEMORY_SYSTEM": "classic", "MEMORY_EMBED_DIMENSIONS": "128"}); err != nil {
		t.Fatal(err)
	}
	if readEmbeddingConfig().Enabled || !readEmbeddingDefaults().Enabled || readEmbeddingConfig().Dimensions != 128 || readEmbeddingDefaults().Dimensions != 256 {
		t.Fatal("preset/global state mixed")
	}
	if err := WriteConfig(map[string]string{}); err != nil {
		t.Fatal(err)
	}
	if !readEmbeddingConfig().Enabled || readEmbeddingConfig().Dimensions != 256 {
		t.Fatal("defaults lost")
	}
	w = httptest.NewRecorder()
	handleEmbeddingMode(w, httptest.NewRequest("POST", "/api/embeddings/mode", strings.NewReader(`{"mode":"classic"}`)))
	if w.Code != 200 || readEmbeddingConfig().Enabled {
		t.Fatal(w.Body.String())
	}
	if readEmbeddingDefaults().Dimensions != 256 {
		t.Fatal("mode toggle erased encoder defaults")
	}
}

func TestEmbeddingTruncationRenormalizesEveryVector(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v := make([]float32, 768)
		v[0] = 3
		v[1] = 4
		v[500] = 12
		json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"index": 0, "embedding": v}, {"index": 1, "embedding": v}}})
	}))
	defer server.Close()
	client := embeddingHTTPClient{URL: server.URL, HTTP: server.Client(), Dimensions: 128}
	rows, err := client.embed(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range rows {
		if len(v) != 128 || v[0] < .599 || v[0] > .601 || v[1] < .799 || v[1] > .801 {
			t.Fatal("truncation/norm", v)
		}
	}
}

func TestEmbeddingDeviceAndModalitiesValidation(t *testing.T) {
	parallelTestHome(t)
	base := embeddingConfig{Device: "gpu", GPUDevice: "ROCm0", Threads: 8, Port: 18081}
	if err := validateEmbeddingConfig(base); err != nil {
		t.Fatal(err)
	}
	args := strings.Join(embeddingArgs(base, "secret"), " ")
	if !strings.Contains(args, "--device ROCm0 --n-gpu-layers 999") {
		t.Fatal(args)
	}
	base.Modalities = "text,image"
	if validateEmbeddingConfig(base) == nil {
		t.Fatal("GGUF accepted unsupported media")
	}
	base.Provider = "transformers"
	base.GPUDevice = "cuda:0"
	if err := validateEmbeddingConfig(base); err != nil {
		t.Fatal(err)
	}
	if _, err := SavePreset("", "bad", "MEMORY_EMBED_MODALITIES=image\n"); err == nil {
		t.Fatal("text-less preset accepted")
	}
}

func TestEmbeddingMediaContainmentAndContentInvalidation(t *testing.T) {
	home := testHome(t)
	root := filepath.Join(home, "memory")
	os.MkdirAll(root, 0700)
	cfg := normalizeEmbeddingConfig(embeddingConfig{Provider: "transformers", Modalities: "text,image"})
	media := filepath.Join(root, "capture.png")
	os.WriteFile(media, []byte("old"), 0600)
	note := "# Capture\n![vue](capture.png)\n"
	os.WriteFile(filepath.Join(root, "note.md"), []byte(note), 0600)
	before, err := loadMemoryDocumentsFor(root, cfg)
	if err != nil {
		t.Fatal(err)
	}
	input, _, err := embeddingMedia(root, note, cfg)
	if err != nil || len(input.Image) != 1 || !strings.Contains(input.Text, "<|image|>") {
		t.Fatal(input, err)
	}
	os.WriteFile(media, []byte("new"), 0600)
	after, err := loadMemoryDocumentsFor(root, cfg)
	if err != nil || before[0].Hash == after[0].Hash || chunkMemory(before)[0].Key == chunkMemory(after)[0].Key {
		t.Fatal("media edit not invalidated", err)
	}
	if _, _, err := embeddingMedia(root, "![escape](../outside.png)", cfg); err == nil {
		t.Fatal("path escape accepted")
	}
	ext := filepath.Join(t.TempDir(), "outside.png")
	os.WriteFile(ext, []byte("private"), 0600)
	if err := os.Symlink(ext, filepath.Join(root, "link.png")); err == nil {
		if _, _, err := embeddingMedia(root, "![link](link.png)", cfg); err == nil {
			t.Fatal("symlink escape accepted")
		}
	}
	_, hash, err := embeddingMedia(root, "![remote](https://example.org/a.png)", cfg)
	if err != nil || hash != "" {
		t.Fatal("network media fetched")
	}
	// Multimodal inputs travel only to the explicit full-model worker.
	if _, err := (&embeddingHTTPClient{}).embedInput(context.Background(), input); err == nil {
		t.Fatal("GGUF accepted multimodal input")
	}
}

// TestEmbeddingMediaRacineSousLienSymbole joue le cas macOS, celui qui faisait
// rougir le runner du 21/11 (ligne 103 : « lien média hors du projet mémoire »).
//
// Sous macOS, /tmp et /var sont des LIENS vers /private/var : t.TempDir() y
// pose donc la racine du projet sous un chemin qui se résout ailleurs. L'image
// est bien dans le projet, mais elle se résout en /private/var/… pendant que la
// racine, elle, restait /var/… : la comparaison les déclarait étrangères.
//
// mediaFace fabrique ce geste sur CHAQUE système (lien symbolique hors Windows,
// nom court 8.3 sous Windows, où le lien exige un droit particulier) : le test
// ne doit pas sauter en silence, sinon rien ne prouve le correctif.
func TestEmbeddingMediaRacineSousLienSymbole(t *testing.T) {
	home := testHome(t)
	reel := filepath.Join(home, "reel")
	racine := filepath.Join(reel, "memory")
	if err := os.MkdirAll(racine, 0o700); err != nil {
		t.Fatal(err)
	}
	face, pourquoi := mediaFace(reel)
	if face == "" {
		t.Skipf("aucun moyen d'obtenir un chemin qui se résout ailleurs : %s", pourquoi)
	}
	// La racine est vue PAR LE LIEN : elle-même ne se résout pas où elle est.
	root := filepath.Join(face, "memory")
	if r, err := filepath.EvalSymlinks(root); err != nil {
		t.Fatalf("la racine par le lien ne se résout pas : %v", err)
	} else if r == root {
		t.Fatalf("la racine par le lien se résout EN ELLE-MÊME (%s) : le cas n'est pas joué", r)
	}
	media := filepath.Join(root, "capture.png")
	if err := os.WriteFile(media, []byte("vue"), 0o600); err != nil {
		t.Fatal(err)
	}
	note := "# Capture\n![vue](capture.png)\n"
	if err := os.WriteFile(filepath.Join(root, "note.md"), []byte(note), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := normalizeEmbeddingConfig(embeddingConfig{Provider: "transformers", Modalities: "text,image"})
	input, hash, err := embeddingMedia(root, note, cfg)
	if err != nil {
		t.Fatalf("média d'un projet sous lien refusé : %v", err)
	}
	if len(input.Image) != 1 || !strings.Contains(input.Text, "<|image|>") {
		t.Fatalf("média non retenu : %+v", input)
	}
	if hash == "" {
		t.Fatal("pas d'empreinte : l'invalidation par contenu ne pourrait pas se faire")
	}
}
