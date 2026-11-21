package aidros

// parallel_peer_files.go — les trois actions fichiers de l'outil peer.
//
// La route existe (parallel_bridge_files.go, posée le 17/11). Sans ces actions, elle est
// atteignable par HTTP et inutilisable par l'agent : l'outil peer ne savait que lister, statut,
// demander, oublier. Mesure du 17/11, chez la voisine à RX 6900 XT : j'ai écrit un script de
// 3 263 octets DANS LE MESSAGE pour le lui poser, et j'ai dû lui dire que le binaire de
// 20 080 640 octets, je ne pouvais pas le lui envoyer.
//
// Ce qui est réutilisé, et pas réécrit : peerHTTPClient (L418, pas de timeout global — la deadline
// est dans le contexte), completionsBase (L260, la racine /v1 du voisin), le canal (canalHeader,
// canalFromHeader, canalHopHeader) pour que le poste d'en face sache qu'un Ajean parle, et la clé
// du voisin (p.Key).
//
// Ce qui N'EST PAS réutilisé, volontairement : peerStatusError (L438) traduit TOUT 404 en « ce
// n'est pas un pont Ajean (surface /v1/* attendue) ». C'est exact pour une complétion, et FAUX pour
// un fichier que le pont nomme mais qu'il n'a pas. Un fichier absent doit être nommé absent, pas
// faire douter de la nature du pont. Les statuts des fichiers ont donc leur propre traduction,
// ci-dessous.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// peerFichiersURL : l'URL de la route fichiers d'un voisin, à partir de sa racine /v1.
func peerFichiersURL(p peerEndpoint, nom string) string {
	base := completionsBase(p.URL) + "/files"
	if nom == "" {
		return base
	}
	return base + "/" + nom
}

// peerFichiersRequete : la requête fichiers, avec ce que le pont attend d'un Ajean.
func peerFichiersRequete(ctx context.Context, p peerEndpoint, methode, nom, corpsEnTete string, corps io.Reader, taille int64) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, methode, peerFichiersURL(p, nom), corps)
	if err != nil {
		return nil, err
	}
	if taille > 0 {
		req.ContentLength = taille
	}
	if corpsEnTete != "" {
		req.Header.Set("Content-Type", corpsEnTete)
	}
	req.Header.Set(canalHeader, "1")
	req.Header.Set(canalFromHeader, canalMachineNom())
	req.Header.Set(canalHopHeader, "1")
	if strings.TrimSpace(p.Key) != "" {
		req.Header.Set("Authorization", "Bearer "+p.Key)
	}
	return req, nil
}

// peerFichiersStatut : un code HTTP de la route fichiers, traduit SANS passer par
// peerStatusError. Le 404 y est la cause la plus fréquente, et la traduction générale y lirait
// « ce n'est pas un pont Ajean » : faux, et surtout il ferait douter de la nature du poste alors
// qu'il nomme juste un fichier absent.
func peerFichiersStatut(p peerEndpoint, code int, corps []byte) error {
	message := bridgeErrorOf(strings.TrimSpace(string(corps)))
	switch code {
	case http.StatusUnauthorized:
		return fmt.Errorf("voisin %s : clé API refusée pour l'échange de fichiers. La clé du canal doit être celle de la machine qui prête (ajean set-api-key là-bas)", p.Name)
	case http.StatusNotFound:
		return fmt.Errorf("voisin %s : le pont répond bien, le fichier demandé n'est pas dans SON dossier d'échange — `files` pour voir ce qu'il a", p.Name)
	case http.StatusConflict:
		return fmt.Errorf("voisin %s : %s — son dossier contient déjà ce nom avec un autre contenu. Demande-lui d'abord, puis renvoie avec le remplacement", p.Name, message)
	case http.StatusRequestEntityTooLarge:
		return fmt.Errorf("voisin %s : %s", p.Name, message)
	case http.StatusBadRequest:
		return fmt.Errorf("voisin %s a refusé ce nom : %s", p.Name, message)
	default:
		return fmt.Errorf("voisin %s : statut %d pour l'échange de fichiers — %s", p.Name, code, message)
	}
}

// peerEnvoiFichier : dépose un fichier LOCAL dans le dossier d'échange du voisin. L'empreinte est
// calculée ICI avant l'envoi et revérifiée sur la réponse : le pont rend celle du fichier qu'il a
// SUR LE DISQUE après relecture, donc l'égalité est la preuve, pas un accusé de réception.
func peerEnvoiFichier(ctx context.Context, p peerEndpoint, cheminLocal string, remplacer bool) (string, error) {
	info, err := os.Stat(cheminLocal)
	if err != nil {
		return "", fmt.Errorf("fichier local illisible (%s) : %w", cheminLocal, err)
	}
	if info.IsDir() {
		return "", fmt.Errorf("%s est un dossier : un seul fichier à la fois", cheminLocal)
	}
	if info.Size() > echangeLimiteOctets {
		return "", fmt.Errorf("fichier local trop volumineux pour le pont : %d octets, %d maximum (%s)", info.Size(), echangeLimiteOctets, cheminLocal)
	}
	nom := filepath.Base(cheminLocal)
	// Le nom part dans l'URL ET dans un en-tête : le pont accepte les deux, l'en-tête est ce qui
	// survive si le nom contient un caractère qu'une URL refuse.
	f, err := os.Open(cheminLocal)
	if err != nil {
		return "", fmt.Errorf("ouverture du fichier local : %w", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("lecture du fichier local pour l'empreinte : %w", err)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("retour au début du fichier local : %w", err)
	}
	ici := hex.EncodeToString(h.Sum(nil))

	entetes := map[string]string{}
	if remplacer {
		entetes[echangeEnTeteRemplacer] = "1"
	}
	req, err := peerFichiersRequete(ctx, p, http.MethodPost, nom, "application/octet-stream", f, info.Size())
	if err != nil {
		return "", err
	}
	req.Header.Set(echangeEnTeteNom, nom)
	req.Header.Set(echangeEnTeteSha, ici)
	for k, v := range entetes {
		req.Header.Set(k, v)
	}
	resp, err := peerHTTPClient.Do(req)
	if err != nil {
		return "", peerDiagnose(p, err)
	}
	defer resp.Body.Close()
	corps, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("lecture de la réponse du voisin %s : %w", p.Name, err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", peerFichiersStatut(p, resp.StatusCode, corps)
	}
	var rendu struct {
		OK           bool   `json:"ok"`
		Nom          string `json:"nom"`
		TailleOctets int64  `json:"taille_octets"`
		Sha256       string `json:"sha256"`
		Error        string `json:"error"`
	}
	if err := json.Unmarshal(corps, &rendu); err != nil {
		return "", fmt.Errorf("réponse d'envoi illisible du voisin %s : %s", p.Name, strings.TrimSpace(string(corps)))
	}
	if !rendu.OK {
		return "", fmt.Errorf("voisin %s : envoi refusé — %s", p.Name, rendu.Error)
	}
	if strings.ToLower(strings.TrimSpace(rendu.Sha256)) != ici {
		return "", fmt.Errorf("voisin %s : empreinte DIFFÉRENTE chez lui. ici %s, là-bas %s — le fichier n'est pas le même, ne pas l'utiliser",
			p.Name, ici, rendu.Sha256)
	}
	if rendu.TailleOctets != info.Size() {
		return "", fmt.Errorf("voisin %s : taille différente chez lui (%d octets contre %d ici)", p.Name, rendu.TailleOctets, info.Size())
	}
	return fmt.Sprintf("[ok] %s fait %s octets, empreinte %s — posé dans le dossier d'échange de %s sous le nom %s, empreinte relue identique là-bas",
		filepath.Base(cheminLocal), fmt.Sprintf("%d", rendu.TailleOctets), ici[:12], p.Name, rendu.Nom), nil
}

// peerListeFichiers : ce que le voisin prête. La DONNÉE est décodée, pas cherchée dans le texte
// (la leçon de preset_moe_ui_test.go:36 : lire le texte brut laisse passer un tableau vide).
func peerListeFichiers(ctx context.Context, p peerEndpoint) (string, error) {
	req, err := peerFichiersRequete(ctx, p, http.MethodGet, "", "", nil, 0)
	if err != nil {
		return "", err
	}
	resp, err := peerHTTPClient.Do(req)
	if err != nil {
		return "", peerDiagnose(p, err)
	}
	defer resp.Body.Close()
	corps, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("lecture de la liste du voisin %s : %w", p.Name, err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", peerFichiersStatut(p, resp.StatusCode, corps)
	}
	fichiers, total, err := echangeDecodageListe(corps)
	if err != nil {
		return "", fmt.Errorf("liste du voisin %s indécodable : %w", p.Name, err)
	}
	if len(fichiers) == 0 {
		return fmt.Sprintf("[ok] le dossier d'échange de %s est vide (%d octets au total)", p.Name, total), nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "le dossier d'échange de %s — %d fichier(s), %d octets :\n", p.Name, len(fichiers), total)
	for _, f := range fichiers {
		nom, _ := f["nom"].(string)
		taille, _ := f["taille_octets"].(float64)
		empreinte, _ := f["sha256"].(string)
		ecrit, _ := f["ecrit_le"].(string)
		if len(empreinte) > 12 {
			empreinte = empreinte[:12]
		}
		fmt.Fprintf(&b, "  %s · %d o · %s · %s\n", nom, int64(taille), empreinte, ecrit)
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

// peerPriseFichier : le fichier du voisin, écrit CHEZ MOI dans mon dossier d'échange, RELU, et
// l'empreinte comparée à celle que le pont annonce. L'écriture et la relecture ne se résument pas
// à « reçu » : c'est la relecture qui a démasqué mes fautes d'encodage (coquilles-shell-windows.md).
func peerPriseFichier(ctx context.Context, p peerEndpoint, nom string) (string, error) {
	nom = strings.TrimSpace(nom)
	if nom == "" {
		return "", errors.New("donner le nom du fichier à récupérer")
	}
	req, err := peerFichiersRequete(ctx, p, http.MethodGet, nom, "", nil, 0)
	if err != nil {
		return "", err
	}
	resp, err := peerHTTPClient.Do(req)
	if err != nil {
		return "", peerDiagnose(p, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		corps, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return "", peerFichiersStatut(p, resp.StatusCode, corps)
	}
	annonce := strings.ToLower(lireEnTeteProtocole(resp.Header, echangeEnTeteSha))

	// Le nom vient du voisin : il passe par le même filtre que celui que le voisin applique à ce
	// que je lui envoie. Sans ça, un voisin de mauvaise foi écrirait hors de mon dossier.
	chezMoi, err := echangeChemin(nom)
	if err != nil {
		return "", fmt.Errorf("le voisin propose un nom que je refuse : %w", err)
	}
	if _, err := os.Stat(chezMoi); err == nil {
		return "", fmt.Errorf("%s existe déjà dans MON dossier d'échange : l'écrasement se décide, pas se subit", filepath.Base(chezMoi))
	}
	if err := os.MkdirAll(echangeDir(), 0o700); err != nil {
		return "", fmt.Errorf("dossier d'échange local : %w", err)
	}
	tmp := chezMoi + ".part"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", fmt.Errorf("écriture locale : %w", err)
	}
	h := sha256.New()
	lu, err := io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, echangeLimiteOctets+1))
	fermeErr := f.Close()
	if err == nil && lu > echangeLimiteOctets {
		err = fmt.Errorf("plus de %d octets reçus", echangeLimiteOctets)
	}
	if err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("réception interrompue : %w", err)
	}
	if fermeErr != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("fermeture du fichier reçu : %w", fermeErr)
	}
	if err := os.Rename(tmp, chezMoi); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("pose locale : %w", err)
	}
	surMonDisque, taille, err := echangeEmpreinte(chezMoi)
	if err != nil {
		return "", fmt.Errorf("reçu et posé mais pas relisible (%s) : %w", chezMoi, err)
	}
	parMoi := hex.EncodeToString(h.Sum(nil))
	if surMonDisque != parMoi {
		return "", fmt.Errorf("empreinte différente après relecture de mon fichier : flux %s, disque %s", parMoi, surMonDisque)
	}
	if annonce != "" && annonce != surMonDisque {
		return "", fmt.Errorf("empreinte DIFFÉRENTE de celle annoncée par %s : lui %s, moi %s — ne pas utiliser ce fichier",
			p.Name, annonce, surMonDisque)
	}
	return fmt.Sprintf("[ok] %s reçu de %s : %d octets, empreinte %s relue identique, posé dans MON dossier d'échange",
		filepath.Base(chezMoi), p.Name, taille, surMonDisque[:12]), nil
}

// peerOubliFichier : le fichier retiré du dossier d'échange du voisin. Une suppression à distance
// se demande, elle ne se fait pas en passant : le nom est rendu, aucun contenu n'est détruit ici.
func peerOubliFichier(ctx context.Context, p peerEndpoint, nom string) (string, error) {
	nom = strings.TrimSpace(nom)
	if nom == "" {
		return "", errors.New("donner le nom du fichier à retirer du dossier d'échange du voisin")
	}
	req, err := peerFichiersRequete(ctx, p, http.MethodDelete, nom, "", nil, 0)
	if err != nil {
		return "", err
	}
	resp, err := peerHTTPClient.Do(req)
	if err != nil {
		return "", peerDiagnose(p, err)
	}
	defer resp.Body.Close()
	corps, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return "", peerFichiersStatut(p, resp.StatusCode, corps)
	}
	return fmt.Sprintf("[ok] %s retiré du dossier d'échange de %s", nom, p.Name), nil
}

// peerFichiersDelai : le fichier se lit et s'écrit, il ne se génère pas. Un envoi de 20 Mio sur le
// réseau local ne relève pas de la deadline d'une génération de texte, et le contexte du tour d'agent
// peut être déjà proche de sa fin. D'où un délai propre, borné.
const peerFichiersDelai = 5 * time.Minute

// peerFichiersBornes : le contexte de la demande fichiers. Avec un délai propre, pour la raison
// ci-dessus ; et avec le contexte du tour en cours comme parent, pour qu'une annulation l'arrête.
func peerFichiersBornes(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return context.WithTimeout(parent, peerFichiersDelai)
}
