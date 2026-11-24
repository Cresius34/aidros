package aidros

// Independent memory encoder. Global defaults live in state; a preset may
// override them without reconfiguring the conversation engine or its mmproj.
import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const embeddingProtocol = "embeddinggemma2-search-v1/chunks-1800-rrf60"

type embeddingConfig struct {
	Enabled    bool   `json:"enabled"`
	Bin        string `json:"bin"`
	Model      string `json:"model"`
	Port       int    `json:"port"`
	Threads    int    `json:"threads"`
	Provider   string `json:"provider"`
	Device     string `json:"device"`
	GPUDevice  string `json:"gpu_device"`
	Modalities string `json:"modalities"`
	Dimensions int    `json:"dimensions"`
}

func readEmbeddingDefaults() embeddingConfig {
	c := embeddingConfig{Port: 18081, Threads: 8}
	getJSON(bkState, "memory_embeddings", &c)
	if parallelBuild() {
		root := filepath.Dir(AjeanHome())
		if filepath.IsAbs(c.Bin) && filepath.Base(filepath.Dir(c.Bin)) == "encoder" {
			c.Bin = filepath.Join("encoder", filepath.Base(c.Bin))
		}
		if filepath.IsAbs(c.Model) && filepath.Base(c.Model) == "embeddinggemma-2-Q8_0.gguf" {
			c.Model = filepath.Join("data", "models", filepath.Base(c.Model))
		}
		if c.Bin != "" && !filepath.IsAbs(c.Bin) {
			c.Bin = filepath.Join(root, c.Bin)
		}
		if c.Model != "" && !filepath.IsAbs(c.Model) {
			c.Model = filepath.Join(root, c.Model)
		}
	}
	return normalizeEmbeddingConfig(c)
}

func readEmbeddingConfig() embeddingConfig {
	return embeddingPresetConfig(readEmbeddingDefaults(), ReadConfig())
}

func saveEmbeddingConfig(c embeddingConfig) error {
	c = normalizeEmbeddingConfig(c)
	if parallelBuild() {
		root := filepath.Dir(AjeanHome())
		paths := []*string{&c.Bin, &c.Model}
		for _, p := range paths {
			if *p == "" {
				continue
			}
			if !filepath.IsAbs(*p) {
				*p = filepath.Join(root, *p)
			}
			if !pathWithin(root, *p) {
				return fmt.Errorf("copier l’encodeur et son modèle dans le dossier portable : %s", *p)
			}
			if err := parallelPackagePath(*p); err != nil {
				return err
			}
			rel, err := filepath.Rel(root, *p)
			if err != nil {
				return err
			}
			*p = rel
		}
	}
	return putJSON(bkState, "memory_embeddings", c)
}

func validateEmbeddingConfig(c embeddingConfig) error {
	c = normalizeEmbeddingConfig(c)
	// Les ports défendus ici ne sont plus des nombres gravés : ce sont ceux que
	//occupe déjà CETTE instance (interface, moteur, pont). 8080/8090 restent
	// réservés parce que c'est l'empreinte d'une installation de production, et
	// 18090/18080 demeurent les défauts — mais une copie décalée sur 18094/18084
	// ne doit plus voir son encodeur refusé pour un chiffre obsolète.
	if c.Port < 1024 || c.Port > 65535 || c.Port == 8080 || c.Port == 8090 ||
		strconv.Itoa(c.Port) == parallelUIPort() || strconv.Itoa(c.Port) == parallelEnginePort() || strconv.Itoa(c.Port) == parallelBridgePort() {
		return errors.New("port encodeur invalide ou déjà pris par cette instance (utiliser 18081)")
	}
	if c.Threads < 1 || c.Threads > 64 {
		return errors.New("threads CPU : 1 à 64")
	}
	if err := validateEmbeddingOptions(c); err != nil {
		return err
	}
	if !c.Enabled {
		return nil
	}
	for i, p := range []string{c.Bin, c.Model} {
		if !filepath.IsAbs(p) {
			return errors.New("choisir un chemin absolu pour le moteur et le modèle")
		}
		fi, err := os.Stat(p)
		wantDirectory := i == 1 && c.Provider == "transformers"
		if err != nil || (wantDirectory && !fi.IsDir()) || (!wantDirectory && !fi.Mode().IsRegular()) {
			return fmt.Errorf("fichier introuvable : %s", p)
		}
	}
	if c.Provider == "transformers" {
		if _, err := os.Stat(filepath.Join(c.Model, "config.json")); err != nil {
			return errors.New("modèle complet absent : lancer Installer-Multimodal.cmd")
		}
		return nil
	}
	if !strings.EqualFold(filepath.Ext(c.Model), ".gguf") {
		return errors.New("le modèle doit être un fichier GGUF")
	}
	return nil
}

// The fingerprint changes when either executable or weights change. It also
// versions the task prefixes and chunker. Vectors are only cached in RAM.
func embeddingFingerprint(c embeddingConfig) (string, error) {
	c = normalizeEmbeddingConfig(c)
	if err := validateEmbeddingConfig(c); err != nil {
		return "", err
	}
	h := sha256.New()
	fmt.Fprintln(h, embeddingProtocol, c.Port, c.Threads, c.Provider, c.Device, c.GPUDevice, c.Modalities, c.Dimensions)
	for _, p := range []string{c.Bin, c.Model} {
		fi, err := os.Stat(p)
		if err != nil {
			return "", err
		}
		fmt.Fprintln(h, filepath.Clean(p), fi.Size(), fi.ModTime().UnixNano())
		if fi.IsDir() {
			err := filepath.WalkDir(p, func(path string, entry os.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if entry.IsDir() {
					if strings.HasPrefix(entry.Name(), ".") {
						return filepath.SkipDir
					}
					return nil
				}
				info, err := entry.Info()
				if err != nil {
					return err
				}
				fmt.Fprintln(h, path, info.Size(), info.ModTime().UnixNano())
				return nil
			})
			if err != nil {
				return "", err
			}
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

type embeddingHTTPClient struct {
	URL, Key, Fingerprint string
	HTTP                  *http.Client
	Dimensions            int
	Provider              string
}

func (c *embeddingHTTPClient) embed(ctx context.Context, input []string) ([][]float32, error) {
	if len(input) == 0 {
		return nil, nil
	}
	return c.embedRequest(ctx, input, len(input))
}

func (c *embeddingHTTPClient) embedRequest(ctx context.Context, input any, count int) ([][]float32, error) {
	// À l'ENTRÉE, pas en différé : une requête qui tourne depuis dix minutes est
	// l'encodeur en train de travailler, donc une marque de vie fraîche. Marquer
	// à la sortie laisserait le veilleur tuer un encodage long.
	markEmbeddingUse(c)
	body, err := json.Marshal(map[string]any{"input": input, "model": "ajean-memory", "encoding_format": "float"})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.URL+"/v1/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Key)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// Do not echo the request, response or source text into logs.
		return nil, fmt.Errorf("encodeur HTTP %d (consulter memory-encoder.log)", resp.StatusCode)
	}
	var result struct {
		Data []struct {
			Index     int       `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&result); err != nil {
		return nil, err
	}
	if len(result.Data) != count {
		return nil, errors.New("nombre de vecteurs inattendu")
	}
	out := make([][]float32, count)
	dim := 0
	for _, item := range result.Data {
		if item.Index < 0 || item.Index >= len(out) || out[item.Index] != nil {
			return nil, errors.New("index de vecteur invalide")
		}
		v := item.Embedding
		// EmbeddingGemma 2 text encoder: 768 dimensions. Refuse another model
		// silently selected by mistake, NaNs and truncated responses.
		if len(v) != 768 || (dim != 0 && len(v) != dim) {
			return nil, errors.New("dimension inattendue (768 attendues pour EmbeddingGemma 2)")
		}
		if c.Dimensions > 0 && c.Dimensions < 768 {
			v = v[:c.Dimensions]
		}
		dim = 768
		norm := 0.0
		for _, x := range v {
			if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
				return nil, errors.New("vecteur non fini")
			}
			norm += float64(x) * float64(x)
		}
		if norm <= 0 {
			return nil, errors.New("vecteur nul")
		}
		norm = math.Sqrt(norm)
		for i := range v {
			v[i] = float32(float64(v[i]) / norm)
		}
		out[item.Index] = v
	}
	return out, nil
}

type embeddingProcess struct {
	cmd     *exec.Cmd
	done    chan struct{}
	err     error // published by closing done
	client  *embeddingHTTPClient
	ready   bool      // protected by embeddingServer.Mutex
	lastUse time.Time // dernière requête servie ; seul le veilleur la lit
}

var embeddingServer struct {
	sync.Mutex
	p         *embeddingProcess
	lastError string
}

func embeddingArgs(c embeddingConfig, key string) []string {
	c = normalizeEmbeddingConfig(c)
	device, layers := "none", "0"
	if c.Device == "gpu" {
		device, layers = c.GPUDevice, "999"
	}
	return []string{"--model", c.Model, "--alias", "ajean-memory", "--embeddings", "--pooling", "mean",
		"--device", device, "--n-gpu-layers", layers, "--host", "127.0.0.1", "--port", strconv.Itoa(c.Port),
		"--threads", strconv.Itoa(c.Threads), "--threads-batch", strconv.Itoa(c.Threads),
		"--ctx-size", "8192", "--batch-size", "8192", "--ubatch-size", "8192", "--parallel", "1", "--api-key", key}
}

func startEmbeddingServer(ctx context.Context, c embeddingConfig) (*embeddingHTTPClient, error) {
	c = normalizeEmbeddingConfig(c)
	if !c.Enabled {
		return nil, errors.New("recherche sémantique désactivée")
	}
	fp, err := embeddingFingerprint(c)
	if err != nil {
		return nil, err
	}
	embeddingServer.Lock()
	p := embeddingServer.p
	if p != nil {
		select {
		case <-p.done:
			embeddingServer.p = nil
			p = nil
		default:
		}
	}
	if p != nil && p.client.Fingerprint != fp {
		killProcessTree(p.cmd)
		embeddingServer.p = nil
		embeddingServer.Unlock()
		select {
		case <-p.done:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return startEmbeddingServer(ctx, c)
	}
	if p == nil {
		if readEmbeddingConfig() != c {
			embeddingServer.Unlock()
			return nil, errors.New("configuration encodeur modifiée")
		}
		// Never adopt, terminate or send memory to an unrelated process on a
		// busy port. Every child also gets its own ephemeral API key.
		// A previous owned child may still be releasing its socket during an
		// immediate app restart. Wait briefly without adopting/killing it.
		if e := waitPortFree("127.0.0.1", strconv.Itoa(c.Port), 5*time.Second); e != nil {
			embeddingServer.Unlock()
			return nil, fmt.Errorf("port encodeur %d occupé ; aucun processus arrêté", c.Port)
		}
		ln, e := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(c.Port)))
		if e != nil {
			embeddingServer.Unlock()
			return nil, fmt.Errorf("port encodeur %d occupé ; aucun processus arrêté", c.Port)
		}
		ln.Close()
		secret := make([]byte, 32)
		if _, e = rand.Read(secret); e != nil {
			embeddingServer.Unlock()
			return nil, e
		}
		key := hex.EncodeToString(secret)
		log, e := os.OpenFile(filepath.Join(AjeanHome(), "memory-encoder.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
		if e != nil {
			embeddingServer.Unlock()
			return nil, e
		}
		args := embeddingArgs(c, key)
		if c.Provider == "transformers" {
			args, e = embeddingTransformerArgs(c, key)
			if e != nil {
				log.Close()
				embeddingServer.Unlock()
				return nil, e
			}
		}
		cmd := hideCmd(exec.Command(c.Bin, args...))
		cmd.Dir = filepath.Dir(c.Bin)
		cmd.Env = libraryPathEnv(cmd.Dir)
		cmd.Stdout, cmd.Stderr = log, log
		prepareBoundChild(cmd)
		if e = cmd.Start(); e != nil {
			log.Close()
			embeddingServer.Unlock()
			return nil, e
		}
		bindChild(cmd)
		p = &embeddingProcess{cmd: cmd, done: make(chan struct{}), lastUse: time.Now(), client: &embeddingHTTPClient{
			URL: fmt.Sprintf("http://127.0.0.1:%d", c.Port), Key: key, Fingerprint: fp, HTTP: &http.Client{Timeout: 30 * time.Second}, Dimensions: c.Dimensions, Provider: c.Provider,
		}}
		embeddingServer.p = p
		go func(p *embeddingProcess) { p.err = p.cmd.Wait(); log.Close(); close(p.done) }(p)
		// Le veilleur démarre ici et pas plus bas : il ne doit veiller que sur
		// l'instance que ce lancement a créée. Une instance reprise (p != nil en
		// entrée de fonction) a déjà le veilleur de son propre lancement.
		go embeddingWatch(p, time.Now())
	}
	if p.ready {
		embeddingServer.Unlock()
		return p.client, nil
	}
	embeddingServer.Unlock()
	probe := &http.Client{Timeout: 700 * time.Millisecond}
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		req, _ := http.NewRequestWithContext(ctx, "GET", p.client.URL+"/health", nil)
		req.Header.Set("Authorization", "Bearer "+p.client.Key)
		if resp, e := probe.Do(req); e == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				embeddingServer.Lock()
				if embeddingServer.p != p {
					embeddingServer.Unlock()
					return nil, errors.New("encodeur remplacé")
				}
				p.ready = true
				embeddingServer.lastError = ""
				embeddingServer.Unlock()
				return p.client, nil
			}
		}
		select {
		case <-p.done:
			return nil, fmt.Errorf("encodeur arrêté (%v) ; moteur compatible gemma-embedding2 requis, voir memory-encoder.log", p.err)
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

// Les deux gardes-mémoire de l'encodeur, calibrées sur celles de la voie
// llama-server (jean_embed.go:68-70) pour que les deux voies obéissent à la même
// politique. La voie « transformers » (Python + torch) était la seule à ne pas
// les observer : mesurée le 14/11/2026 sur le poste 20.1, son encodeur retenait
// 24,8 Go privés et 15,9 Go vifs APRÈS la fin de l'indexation (0,1 s de CPU sur
// 25 s), sans jamais les rendre, jusqu'à l'arrêt de l'application.
const (
	embeddingStopRAMMB = 700 // sous ce seuil, l'encodeur est arrêté
	// embeddingIdleStop : 0 = JAMAIS arrêté pour inactivité.
	//
	// ⭐ La garde est LEVÉE sur l'ordre de l'humain du 24/11/2026 : « pas de garde
	// d'inactivité, si Aidros est lancé, la mémoire reste indexée et s'actualise
	// au fur et à mesure ». Ce n'est pas un caprice, c'est une mesure : la garde
	// de vingt minutes protégeait la machine d'une fuite de 24,8 Go, mais elle
	// protégeait la voie PYTHON. La voie en service (llama.cpp/GGUF, mesuré
	// pid 133460 le 24/11) tient 426 Mo vifs / 1 299 Mo privés, soit ~2 % de ce
	// pour quoi la garde a été écrite. Et son coût, lui, n'a pas baissé : tuer
	// l'encodeur rend la recherche suivante lexicale (readyEmbeddingClient répond
	// « encodeur non démarré », mem_embedding_backend.go:527) et, comme le cache
	// disque ne s'écrivait pas, le plein réencodage — ~12 min de CPU pour
	// 1 561 fragments à 0,47 s.
	//
	// ⚠ CE QUE LA GARDE NE PROTÉGEAIT PAS, et qui reste protégé : la RAM. Le
	// seuil embeddingStopRAMMB, lui, veille toujours — et c'est LUI qui était en
	// cause le 14/11 (5,5 Go libres mesurés), jamais l'inactivité (mesurée 15,4 à
	// 15,9 Go LIBRES aujourd'hui, seuil hors d'atteinte).
	//
	// ⚠ ZÉRO N'EST PAS « JAMAIS » TOUT SEUL : `time.Since(...) > 0` est VRAI
	// aussitôt. Les deux comparateurs (ici :459 et jean_embed.go:260) testent
	// d'abord si la garde est ARMÉE. Retirer ce test-là tuerait l'encodeur à son
	// premier battement — la garde levée tuerait plus vite que la garde armée.
	embeddingIdleStop = 0 * time.Minute
	// Le veilleur ne regarde pas avant ce délai. Le premier lancement de la voie
	// Python installe torch et ses dépendances, qui dépasse largement une phase
	// d'encodage et consomme justement de la RAM : sans cette tolérance, on
	// tuerait une installation légitime en plein travail.
	embeddingGrace = 45 * time.Minute
)

// markEmbeddingUse note que cet encodeur vient de servir. lastUse est protégé
// par embeddingServer.Mutex, la même serrure que le veilleur.
//
// embedRequest est le SEUL point de passage de toute la voie encodeur :
// embed() (texte) et embedInput() (multimodal) y aboutissent tous les deux
// (mem_embedding_media.go:126,131), et la boucle d'indexation y passe pour
// chaque fragment (mem_embeddings.go:252). Marquer ici protège donc une
// indexation en cours contre l'arrêt pour inactivité.
func markEmbeddingUse(c *embeddingHTTPClient) {
	embeddingServer.Lock()
	if p := embeddingServer.p; p != nil && p.client == c {
		p.lastUse = time.Now()
	}
	embeddingServer.Unlock()
}

// embeddingWatchDecision rend la décision du veilleur, sans processus ni RAM :
// une fonction pure, donc testable. now lui est passé pour qu'un test fixe le
// temps au lieu d'attendre vingt minutes.
//
// availMB vaut -1 quand la RAM est indisponible : on ne tue pas sur un doute.
//
// idleStop est passé EN PARAMÈTRE et n'est pas lu dans la constante : la garde
// d'inactivité est un RÉGLAGE dont la valeur du jour est 0 (levée). Un filet qui
// ne lirait que la constante ne prouverait qu'un seul état ; avec le réglage en
// entrée, le même test joue la garde armée ET la garde levée, et mord dans les
// deux sens. C'est la seule façon de tester « jamais » sans attendre.
//
// ⚠ idleStop <= 0 = la garde n'est PAS armée. Le test d'armement vient AVANT la
// soustraction : sans lui, `now.Sub(lastUse) > 0` serait vrai aussitôt et une
// garde levée tuerait l'encodeur plus vite qu'une garde de vingt minutes.
func embeddingWatchDecision(now, started, lastUse time.Time, availMB int, idleStop time.Duration) (bool, string) {
	if now.Sub(started) <= embeddingGrace {
		return false, "" // tolérance de démarrage : torch s'installe, il travaille
	}
	if a := availMB; a >= 0 && a < embeddingStopRAMMB {
		return true, fmt.Sprintf("RAM trop basse (%d Mo libres, %d requis)", a, embeddingStopRAMMB)
	}
	if idleStop > 0 {
		if d := now.Sub(lastUse); d > idleStop {
			return true, fmt.Sprintf("inutilisé depuis %s", d.Truncate(time.Minute))
		}
	}
	return false, ""
}

// embeddingWatch surveille l'encodeur « transformers » (Python) et l'arrête
// quand la machine en a besoin ou qu'il ne sert plus. C'est l'équivalent du
// watch() de jean_embed.go:268-287, que cette voie n'avait pas :
// stopEmbeddingServer n'était appelé que par web_embeddings (à la demande),
// sys_tray_windows, parallel_profile (à l'arrêt) et backend_presets.
//
// Tuer en pleine indexation aurait un coût : stopEmbeddingServer appelle
// clearEmbeddingCache, qui jette les vecteurs déjà encodés (mesuré : 693 blocs
// sur 45 pages). C'est markEmbeddingUse qui l'en empêche, pas une inspection du
// job : l'indexation parle au même encodeur, donc sa dernière requête est récente.
func embeddingWatch(p *embeddingProcess, started time.Time) {
	for {
		select {
		case <-p.done:
			return // mort de lui-même, ou arrêté volontairement : plus rien à surveiller
		default:
		}

		embeddingServer.Lock()
		vivante := embeddingServer.p == p
		lastUse := p.lastUse
		embeddingServer.Unlock()
		if !vivante {
			return // une autre instance a pris la main : ce n'est plus à elle de veiller
		}

		if stop, why := embeddingWatchDecision(time.Now(), started, lastUse, availRAMMB(), embeddingIdleStop); stop {
			fmt.Fprintf(os.Stderr, "[mémoire] encodeur arrêté : %s\n", why)
			stopEmbeddingServer()
			return
		}
		time.Sleep(2 * time.Second)
	}
}

func stopEmbeddingServer() {
	embeddingServer.Lock()
	p := embeddingServer.p
	embeddingServer.p = nil
	embeddingServer.Unlock()
	if p != nil {
		select {
		case <-p.done:
		default:
			killProcessTree(p.cmd)
		}
		select {
		case <-p.done:
		case <-time.After(5 * time.Second):
		}
	}
	clearEmbeddingCache()
}

func readyEmbeddingClient(c embeddingConfig) (*embeddingHTTPClient, error) {
	fp, err := embeddingFingerprint(c)
	if err != nil {
		return nil, err
	}
	embeddingServer.Lock()
	defer embeddingServer.Unlock()
	p := embeddingServer.p
	if p == nil || !p.ready || p.client.Fingerprint != fp {
		return nil, errors.New("encodeur non démarré : ouvrir Mémoire sémantique")
	}
	select {
	case <-p.done:
		return nil, errors.New("encodeur arrêté : redémarrer dans Mémoire sémantique")
	default:
	}
	return p.client, nil
}
