package aidros

// parallel_bridge_files.go — l'échange de FICHIERS par le pont Ajean↔Ajean.
// Une seule clause package dans ce fichier : la seconde, mesurée go vet, casse la compilation.
//
// Pourquoi : le pont ne transportait QUE du texte. Mesuré le 17/11/2026, chez la voisine à
// RX 6900 XT : pour lui poser un script de 3 263 octets, j'ai dû l'écrire DANS LE MESSAGE —
// 3,3 Ko dans une complétion, avec le risque qu'il soit recopié mal ou récité sans être joué
// (mesuré quatre fois). Et pour le nouveau binaire, 20 080 640 octets, la voie texte n'existe
// pas : « je ne peux pas déposer le fichier » a été la réponse honnête. Tout le chantier « poser
// l'exe là-bas » est bloqué là.
//
// Choix, dans l'ordre :
//   - UN SEUL DOSSIER, data\echange\. La route ne connaît que des noms PLATS. Le poste d'en face
//     ne voit jamais un chemin absolu du poste d'ici, et ne peut pas en envoyer un non plus.
//   - la clé API de la machine reste la seule authentification : la route est branchée APRÈS le
//     contrôle existant (antKeyOK, parallel_bridge.go:181), AVANT le canal (L191). Sans ça, un
//     appel portant l'en-tête du canal se ferait avaler par un tour d'agent (mesuré : c'est la
//     première branche après l'auth).
//   - SHA-256 rendu à l'émission ET revérifié à la réception, fichier RELU après écriture. Un
//     fichier sans son empreinte n'est pas livrable — c'est la règle de la paire (exe + BUILD.json).
//   - taille MAXIMALE affichée : la dépasser est une ERREUR qui le dit, jamais un troncage
//     silencieux. Un log de 70 000 lignes ne doit pas pouvoir entrer par là.
//   - rien n'écrasé en silence : un fichier déjà présent avec un contenu DIFFÉRENT répond 409, et
//     l'en-tête X-Aidros-Remplacer est demandé pour passer outre.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// limite d'un seul échange : 32 Mio. L'exe en service fait 20 080 640 o et le paquet d'atelier
// complet 8 020 615 o : les deux passent. Un journal de 70 000 lignes (le plus gros que j'aie lu
// cette semaine) en fait partie aussi, volontairement.
const echangeLimiteOctets int64 = 32 << 20

// echangeNomMax : un nom plat, pas un chemin, et pas un pavé.
const echangeNomMax = 128

const echangeEnTeteRemplacer = "X-Aidros-Remplacer"
const echangeEnTeteSha = "X-Aidros-Sha256"
const echangeEnTeteNom = "X-Aidros-Fichier"

// echangeDir : le dossier d'échange, sous le dossier de données du poste. Calé comme moeHome()
// et presetsDir() (backend_moe.go:188, run.go:262) : même racine, même comportement en portable.
func echangeDir() string { return filepath.Join(AjeanHome(), "echange") }

// echangeRoute : la surface du pont n'est QUE /v1/* (parallel_bridge.go:168 rejette tout le reste
// par un 404). La route fichiers doit donc vivre SOUS /v1/, sinon elle est écartée avant d'exister.
func echangeRoute(p string) bool {
	return p == "/v1/files" || strings.HasPrefix(p, "/v1/files/")
}

// echangeNomAssaini : le nom vient du réseau, donc il est coupable jusqu'à preuve du contraire.
// Refusé : vide, trop long, tout séparateur, tout ce qui peut sortir du dossier, tout caractère
// de contrôle, et tout nom qui change d'identité une fois nettoyé. bbolt pose un verrou EXCLUSIF
// sur la base (store.go:46-53) : laisser « ..\..\bddtchat.db » passer, c'est donner à un voisin le
// moyen de lire la base active par le réseau — exactement le geste qui fait crier « corrompu ».
func echangeNomAssaini(nom string) (string, error) {
	n := strings.TrimSpace(nom)
	if n == "" {
		return "", errors.New("nom de fichier absent")
	}
	if len(n) > echangeNomMax {
		return "", fmt.Errorf("nom trop long (%d octets, %d maximum)", len(n), echangeNomMax)
	}
	if strings.ContainsAny(n, `/\:*?"<>|`) {
		return "", fmt.Errorf("nom refusé : un séparateur ou un caractère réservé (%s)", n)
	}
	if n == "." || n == ".." || strings.HasPrefix(n, ".") {
		// Le point d'entrée : un nom qui commence par un point est un nom caché ou un raccourci
		// de chemin. Le dossier d'échange n'a rien à cacher.
		return "", fmt.Errorf("nom refusé : commence par un point ou désigne un dossier (%s)", n)
	}
	if filepath.IsAbs(n) || filepath.VolumeName(n) != "" {
		return "", fmt.Errorf("nom refusé : chemin absolu ou lecteur (%s)", n)
	}
	for _, r := range n {
		if r < 0x20 || r == 0x7f {
			return "", errors.New("nom refusé : caractère de contrôle")
		}
	}
	if cleaned := filepath.Base(filepath.Clean(n)); cleaned != n {
		return "", fmt.Errorf("nom refusé : nettoyé il change (%s devient %s)", n, cleaned)
	}
	return n, nil
}

// echangeChemin : le nom_assaini + le dossier, avec le contrôle du chemin résolu. Le filtre sur le
// nom doit suffire ; on le revérifie quand même, parce qu'un seul oubli dans un filtre écrit à
// l'emporte-pièce ouvre le dossier entier.
func echangeChemin(nom string) (string, error) {
	n, err := echangeNomAssaini(nom)
	if err != nil {
		return "", err
	}
	dir := echangeDir()
	p := filepath.Join(dir, n)
	rel, err := filepath.Rel(dir, p)
	if err != nil || rel == "" || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("chemin hors du dossier d'échange : %s", n)
	}
	return p, nil
}

// echangeEmpreinte : le SHA-256 d'un fichier sur le disque.
func echangeEmpreinte(chemin string) (string, int64, error) {
	f, err := os.Open(chemin)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

// handleEchangeListe : ce que le poste prête, sans rien révéler de ses chemins.
func handleEchangeListe(w http.ResponseWriter) {
	dir := echangeDir()
	out := []map[string]any{}
	entrees, err := os.ReadDir(dir)
	if err != nil && !os.IsNotExist(err) {
		sendJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	var total int64
	for _, e := range entrees {
		if e.IsDir() {
			continue
		}
		chemin := filepath.Join(dir, e.Name())
		info, err := e.Info()
		if err != nil {
			continue
		}
		empreinte, taille, err := echangeEmpreinte(chemin)
		if err != nil {
			continue
		}
		total += taille
		out = append(out, map[string]any{
			"nom":          e.Name(),
			"taille_octets": taille,
			"sha256":       empreinte,
			"ecrit_le":     info.ModTime().Format(time.RFC3339),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		a, _ := out[i]["nom"].(string)
		b, _ := out[j]["nom"].(string)
		return a < b
	})
	sendJSON(w, http.StatusOK, map[string]any{
		"ok":             true,
		"dossier":        "echange",
		"entrees":        len(out),
		"total_octets":   total,
		"limite_octets":  int64(echangeLimiteOctets),
		"fichiers":       out,
	})
}

// handleEchangeGet : le fichier demandé, en octets, avec son empreinte dans un en-tête.
func handleEchangeGet(w http.ResponseWriter, r *http.Request) {
	nom := strings.TrimPrefix(r.URL.Path, "/v1/files/")
	p, err := echangeChemin(nom)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	empreinte, taille, err := echangeEmpreinte(p)
	if err != nil {
		if os.IsNotExist(err) {
			sendJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "fichier absent du dossier d'échange"})
			return
		}
		sendJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	w.Header().Set(echangeEnTeteSha, empreinte)
	w.Header().Set(echangeEnTeteNom, filepath.Base(p))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", taille))
	w.WriteHeader(http.StatusOK)
	f, err := os.Open(p)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = io.Copy(w, f)
}

// handleEchangePost : reçoit des octets, les écrit, RELIT, et rend l'empreinte de ce qui est sur
// le disque. La relecture est ce qui prouve, pas l'écriture : c'est elle qui a démasqué mes fautes
// d'encodage (coquilles-shell-windows.md), et c'est elle qui rend la PAIRE possible de l'autre bord.
func handleEchangePost(w http.ResponseWriter, r *http.Request) {
	nom := strings.TrimPrefix(r.URL.Path, "/v1/files/")
	if nom == "" || nom == r.URL.Path {
		nom = strings.TrimSpace(r.Header.Get(echangeEnTeteNom))
	}
	p, err := echangeChemin(nom)
	if err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	// La limite est opposée AUSSITÔT que Content-Length est connu, et pendant la lecture sinon :
	// un POST sans Content-Length ne doit pas pouvoir écrire 4 Go avant qu'on le remarque.
	if r.ContentLength > echangeLimiteOctets {
		sendJSON(w, http.StatusRequestEntityTooLarge, map[string]any{
			"ok": false, "error": fmt.Sprintf("fichier trop volumineux : %d octets, %d maximum", r.ContentLength, echangeLimiteOctets),
		})
		return
	}
	if r.ContentLength == 0 {
		sendJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "aucun octet reçu"})
		return
	}
	if err := os.MkdirAll(echangeDir(), 0o700); err != nil {
		sendJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	// Écrasement : jamais en silence. Un fichier déjà là, avec un autre contenu, répond 409.
	if info, err := os.Stat(p); err == nil && !info.IsDir() {
		ancien, _, _ := echangeEmpreinte(p)
		enTete := strings.ToLower(lireEnTeteProtocole(r.Header, echangeEnTeteRemplacer))
		if enTete != "1" && enTete != "true" && enTete != "oui" {
			sendJSON(w, http.StatusConflict, map[string]any{
				"ok": false, "nom": filepath.Base(p), "sha256": ancien,
				"taille_octets": info.Size(),
				"error":         "fichier déjà présent avec un autre contenu : en-tête " + echangeEnTeteRemplacer + ": 1 pour remplacer",
			})
			return
		}
	}
	// On écrit dans un .part puis on renomme : un voisin qui lirait pendant l'écriture ne verrait
	// pas un fichier à moitié plein passé pour complet.
	tmp := p + ".part"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		sendJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	h := sha256.New()
	lu, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(r.Body, echangeLimiteOctets+1))
	fermeErr := f.Close()
	if err == nil && lu > echangeLimiteOctets {
		err = fmt.Errorf("fichier trop volumineux : plus de %d octets", echangeLimiteOctets)
	}
	if err != nil {
		_ = os.Remove(tmp)
		code := http.StatusServiceUnavailable
		if strings.Contains(err.Error(), "volumineux") {
			code = http.StatusRequestEntityTooLarge
		}
		sendJSON(w, code, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	if fermeErr != nil {
		_ = os.Remove(tmp)
		sendJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": fermeErr.Error()})
		return
	}
	if err := os.Rename(tmp, p); err != nil {
		_ = os.Remove(tmp)
		sendJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	// RELITURE : l'empreinte rendue est celle du fichier SUR LE DISQUE, pas celle du flux reçu.
	relu, taille, err := echangeEmpreinte(p)
	if err != nil {
		sendJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": "posé mais relisible : " + err.Error()})
		return
	}
	recu := hex.EncodeToString(h.Sum(nil))
	if relu != recu {
		sendJSON(w, http.StatusServiceUnavailable, map[string]any{
			"ok": false, "error": "empreinte différente après relecture", "recu": recu, "sur_le_disque": relu,
		})
		return
	}
	sendJSON(w, http.StatusOK, map[string]any{
		"ok": true, "nom": filepath.Base(p), "taille_octets": taille, "sha256": relu,
	})
}

// handleEchange : l'aiguillage. La route est branchée dans parallelBridgeHandler APRÈS le contrôle
// de clé et AVANT canalDemande — sinon l'appel portant l'en-tête du canal part dans un tour d'agent
// et l'upload est avalé comme un message (parallel_bridge.go:191).
func handleEchange(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if r.URL.Path == "/v1/files" {
			handleEchangeListe(w)
			return
		}
		handleEchangeGet(w, r)
	case http.MethodPost, http.MethodPut:
		handleEchangePost(w, r)
	case http.MethodDelete:
		if r.URL.Path == "/v1/files" {
			sendJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "suppression : donner le nom du fichier"})
			return
		}
		p, err := echangeChemin(strings.TrimPrefix(r.URL.Path, "/v1/files/"))
		if err != nil {
			sendJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		if err := os.Remove(p); err != nil {
			if os.IsNotExist(err) {
				sendJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": "fichier absent"})
				return
			}
			sendJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		sendJSON(w, http.StatusOK, map[string]any{"ok": true, "nom": filepath.Base(p)})
	default:
		w.Header().Set("Allow", "GET, POST, PUT, DELETE")
		sendJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "error": "méthode non admise sur le dossier d'échange"})
	}
}

// echangeDecodageListe : lit la réponse de liste côté appelant. Séparé pour que le test porte sur
// la DONNÉE, pas sur le texte brut — la leçon de preset_moe_ui_test.go:36 (décoder une enveloppe
// qui n'existe pas laisse la liste vide et le test passe sans rien vérifier).
func echangeDecodageListe(corps []byte) ([]map[string]any, int64, error) {
	var rep struct {
		OK           bool             `json:"ok"`
		Error        string           `json:"error"`
		TotalOctets  int64            `json:"total_octets"`
		Fichiers     []map[string]any `json:"fichiers"`
	}
	if err := json.Unmarshal(corps, &rep); err != nil {
		return nil, 0, err
	}
	if !rep.OK {
		return nil, 0, fmt.Errorf("refusé par le pair : %s", rep.Error)
	}
	return rep.Fichiers, rep.TotalOctets, nil
}
