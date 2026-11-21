// backend_llamacpp_helpers.go : les 7 utilitaires que le reste du projet
// reclame a backend_llamacpp.go et backend_prebuilt.go, supprimes par la
// fusion amont 0.20.1. Deplaces ici au lieu de garder les 715 + 550 lignes
// des deux fichiers : plus personne n appelle cmdLlamacpp, requireTools,
// buildBackend ni prebuiltResolveBin (mesure : aucun appel dans l arbre).
// Le GPU Cloud (Modal) n est pas repris : brique enterree par l amont, et ce
// poste tourne sous ROCm gfx1201 local. Ses 4 references ont ete retirees.
package aidros

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)


// backendDir renvoie backends/<name>, en refusant tout nom qui s'échapperait du
// dossier backends (../, chemin absolu…).
func backendDir(name string) (string, error) {
	name = sanitizeBackendName(name)
	root, err := filepath.Abs(backendsDir())
	if err != nil {
		return "", err
	}
	p := filepath.Join(root, name)
	if !strings.HasPrefix(p, root+string(filepath.Separator)) {
		return "", fmt.Errorf("nom de backend invalide")
	}
	if parallelBuild() {
		if err := parallelWritablePath(p); err != nil {
			return "", err
		}
	}
	return p, nil
}


// buildPlan capture les flags CMake adaptés à la machine courante.
type buildPlan struct {
	backend  string // "cuda" | "hip" | "metal" | "vulkan" | "cpu"
	cudaArch string // ex. "120" ou "86;89" (vide => détection native par CMake)
	cudaCXX  string // chemin de nvcc quand backend == cuda
	cudaVer  string // version du toolkit retenu, ex. "12.8" (vide si inconnue)
	// cudaArchUnsupported : le GPU détecté est plus vieux que ce que le SEUL
	// toolkit CUDA installé sait compiler (typiquement un GPU Pascal/Volta avec
	// CUDA 13, qui a supprimé ces archs). Le build CUDA est voué à casser sur le
	// « simple test program » de CMake → buildLlamacpp avertit clairement au lieu
	// de laisser nvcc échouer sans explication.
	cudaArchUnsupported bool
	flags               []string // flags -D… passés à `cmake -B build`
	jobs                int      // parallélisme du build
	gen                 string   // générateur CMake (-G), vide => défaut de la plateforme
	genArch             string   // architecture du générateur (-A), ex. "x64" (VS uniquement)
	forced              bool     // backend imposé par --backend (pas de conseil « GPU non utilisé »)
}


func defaultRepoDir() string {
	return filepath.Join(backendsDir(), "llama.cpp")
}


// ---------------------------------------------------------------------------
// Localisation du dépôt
// ---------------------------------------------------------------------------

// llamacppRepoDir resolves the llama.cpp checkout: derived from config BIN when
// possible (so `update` targets whatever build the service actually runs),
// otherwise the default under $AJEAN_HOME/backends/llama.cpp.
func llamacppRepoDir() string {
	if parallelBuild() {
		return defaultRepoDir()
	} // shared BIN is execution-only
	if bin := ReadConfig()["BIN"]; bin != "" {
		if real, err := filepath.EvalSymlinks(bin); err == nil {
			bin = real
		}
		if root := findRepoRoot(bin); root != "" {
			return root
		}
	}
	return defaultRepoDir()
}


// sanitizeBackendName réduit un nom à un identifiant de dossier sûr
// ([a-z0-9._-]), pour qu'il ne puisse jamais s'échapper de backends/.
func sanitizeBackendName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		case r == ' ' || r == '/' || r == '\\':
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), ".-")
	if out == "" {
		return "backend"
	}
	return out
}


// findRepoRoot walks up from a binary path (…/build/bin/llama-server) looking
// for the llama.cpp source root (a dir holding .git or CMakeLists.txt).
func findRepoRoot(binPath string) string {
	d := filepath.Dir(binPath)
	for i := 0; i < 6; i++ {
		if isDir(filepath.Join(d, ".git")) || isFile(filepath.Join(d, "CMakeLists.txt")) {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return ""
}


// downloadWithProgress télécharge url vers dest en journalisant la progression
// par tranches de ~25 Mo.
func downloadWithProgress(url, dest string, total int64, logf func(string)) error {
	client := &http.Client{Timeout: 0}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if total <= 0 {
		total = resp.ContentLength
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	buf := make([]byte, 1<<20)
	var done, lastLog int64
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return werr
			}
			done += int64(n)
			if done-lastLog >= 25<<20 {
				lastLog = done
				if total > 0 {
					logf(fmt.Sprintf("⬇ %d / %d Mo (%d%%)", done/1_000_000, total/1_000_000, done*100/total))
				} else {
					logf(fmt.Sprintf("⬇ %d Mo", done/1_000_000))
				}
			}
		}
		if rerr == io.EOF {
			return nil
		}
		if rerr != nil {
			return rerr
		}
	}
}

