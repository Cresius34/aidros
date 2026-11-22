package aidros

// La preuve qui manque aux deux commits précédents.
//
// parallel_bridge_files_test.go appelle le HANDLER (httptest.NewRecorder) : il prouve la route,
// l'aiguillage, l'assainissement. parallel_peer_files.go n'était couvert par AUCUN test : ses
// fonctions construisent une vraie requête HTTP vers un voisin, et rien ici ne parlait à un voisin.
// Un transport qui n'a jamais été joué, c'est exactement ce que j'ai reproché à mon installeur :
// « posé, jamais rejoué là-bas ».
//
// Le montage est celui de TestParallelBridgeEndToEndTwoInstances:206 — le pont de B sur une vraie
// socket TCP, A ne connaissant que son URL et sa clé.
//
// ⚠ CE QUE CE TEST NE Prouve PAS, et c'est à écrire : A et B sont dans le MÊME processus, donc ils
// partagent le même AjeanHome() et le MÊME dossier d'échange (le test de bout en bout signale la
// même limite pour la config, L288-292). La séparation des deux dossiers, un de chaque côté du
// réseau, ne se voit que sur deux machines. Ce test prouve le TRANSPORT, l'EMPREUVE AUX DEUX BORDS
// et le REFUS d'écrasement ; pas l'isolation des disques.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestEchangeFichiersCycleReelDeuxInstances(t *testing.T) {
	parallelTestHome(t)
	echangeTestCle(t) // la clé de B, celle que le pont exige

	// --- B : son pont sur TCP ---
	bridgeLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("aucun port libre pour ce test")
	}
	bridgePort := strconv.Itoa(bridgeLn.Addr().(*net.TCPAddr).Port)
	pontB := &http.Server{Handler: parallelBridgeHandler()}
	go func() { _ = pontB.Serve(bridgeLn) }()
	defer pontB.Close()

	// --- A : un voisin déclaré, comme l'outil le fait ---
	p, err := peerAddDeclared("B", "http://127.0.0.1:"+bridgePort+"/v1", "sk-echange-test", "")
	if err != nil {
		t.Fatal(err)
	}

	// --- le fichier à envoyer, HORS du dossier d'échange (il est partagé ici, voir l'en-tête) ---
	local := filepath.Join(t.TempDir(), "piece-de-B.bin")
	contenu := strings.Repeat("octet pour octet, la piece qui manquait chez la voisine. ", 47) + "\n"
	if err := os.WriteFile(local, []byte(contenu), 0o600); err != nil {
		t.Fatal(err)
	}
	somme := sha256.Sum256([]byte(contenu))
	attendu := hex.EncodeToString(somme[:])

	ctx, cancel := peerFichiersBornes(context.Background())
	defer cancel()

	// --- 1) ENVOI, et l'empreinte relue par B ---
	msg, err := peerEnvoiFichier(ctx, p, local, false)
	if err != nil {
		t.Fatalf("envoi : %v", err)
	}
	if !strings.Contains(msg, attendu[:12]) || !strings.Contains(msg, "[ok]") {
		t.Errorf("envoi mal rendu : %s (empreinte attendue %s)", msg, attendu[:12])
	}
	// Le fichier est chez B — ici le même dossier, et c'est justement ce qu'on contrôle.
	chezB := filepath.Join(echangeDir(), "piece-de-B.bin")
	if b, err := os.ReadFile(chezB); err != nil {
		t.Fatalf("envoyé mais absent du dossier d'échange de B : %v", err)
	} else if string(b) != contenu {
		t.Errorf("octets de B différents de l'original (%d contre %d)", len(b), len(contenu))
	}

	// --- 2) ÉCRASEMENT REFUSÉ, sans y toucher deux fois à la main ---
	if _, err := peerEnvoiFichier(ctx, p, local, false); err == nil {
		t.Errorf("deuxième envoi du même nom accepté sans le drapeau remplacer : l'écrasement silencieux est revenu")
	} else if !strings.Contains(err.Error(), "409") && !strings.Contains(strings.ToLower(err.Error()), "déjà") {
		t.Errorf("refus d'écrasement mal nommé : %v", err)
	}
	// Avec le drapeau, ça passe.
	if _, err := peerEnvoiFichier(ctx, p, local, true); err != nil {
		t.Errorf("envoi avec remplacer: true refusé : %v", err)
	}

	// --- 3) LISTE, et c'est la DONNÉE qui est contrôlée ---
	liste, err := peerListeFichiers(ctx, p)
	if err != nil {
		t.Fatalf("liste : %v", err)
	}
	if !strings.Contains(liste, "piece-de-B.bin") || !strings.Contains(liste, attendu[:12]) {
		t.Errorf("liste du voisin ne rend pas le nom et l'empreinte :\n%s", liste)
	}
	if !strings.Contains(liste, strconv.Itoa(len(contenu))) {
		t.Errorf("liste : la taille n'y est pas (%d octets attendus)\n%s", len(contenu), liste)
	}

	// --- 4) LA PRISE, et ce qu'elle peut être contrôlée ICI ---
	// ⚠ CE QUE LE TEST NE PEUT PAS JOUER, et c'est une découverte sur mon propre code, mesurée ce
	// 17/11 en tombant dessus : A et B partagent le MÊME AjeanHome(), donc le MÊME dossier d'échange.
	// Retirer le fichier « chez B » le retire aussi chez moi ; et la prise refuse un nom déjà présent
	// chez moi. Les deux gardes étant dans le même dossier, LE CHEMIN DE SUCCÈS de peerPriseFichier
	// est injouable en un seul processus : il lui faut deux machines. Premier essai, mon test échouait
	// sur un 404 PARFAITEMENT JUSTE — c'est lui qui avait tort, pas la route.
	// Ce qui EST joué ici : le 404 nommé, le refus d'écrasement chez moi, le nom à séparateurs, la
	// clé refusée. Ce qui ne l'est pas : les octets qui reviennent et leur empreinte relue.
	if _, err := peerPriseFichier(ctx, p, "piece-de-B.bin"); err == nil {
		t.Errorf("prise sur un nom déjà présent acceptée : l'écrasement chez MOI est silencieux")
	} else if !strings.Contains(err.Error(), "existe déjà") {
		t.Errorf("refus d'écrasement local mal nommé : %v", err)
	}
	// Un nom que B n'a pas : nommé ABSENT, jamais « pas un pont Aidros ».
	if _, err := peerPriseFichier(ctx, p, "jamis-envoye-depuis-toujours.bin"); err == nil {
		t.Errorf("prise d'un fichier absent acceptée")
	} else if strings.Contains(err.Error(), "pas un pont Aidros") {
		t.Errorf("fichier absent traduit en « pas un pont Aidros » : la traduction générale est revenue\n%v", err)
	} else if !strings.Contains(err.Error(), "dossier d'échange") {
		t.Errorf("fichier absent mal nommé : %v", err)
	}

	// --- 5) LE NOM À SÉPARATEURS EST REFUSÉ ---
	// HONNÊTE : ce cas ne peut PAS départager « refusé par moi » de « refusé par B », puisque B
	// applique le même filtre avant moi (c'est le même code). Ce qu'il prouve, c'est qu'un nom à
	// séparateurs ne passe PAS et n'écrit rien.
	if _, err := peerPriseFichier(ctx, p, `..\..\ajean.db`); err == nil {
		t.Errorf("un nom à separateurs accepté : le filtre ne s'applique qu'à l'envoi")
	} else if !strings.Contains(err.Error(), "refus") {
		t.Errorf("refus du nom mal nommé : %v", err)
	}

	// --- 6) RETRAIT chez le voisin ---
	if _, err := peerOubliFichier(ctx, p, "piece-de-B.bin"); err != nil {
		t.Fatalf("retrait : %v", err)
	}
	if _, err := os.Stat(filepath.Join(echangeDir(), "piece-de-B.bin")); !os.IsNotExist(err) {
		t.Errorf("retiré annoncé mais toujours présent")
	}
	// Après retrait, la liste répond et rend un dossier vide : la route n'est pas cassée par un
	// dossier vide (elle doit répondre, pas 503).
	liste2, err := peerListeFichiers(ctx, p)
	if err != nil {
		t.Errorf("liste après retrait : %v", err)
	} else if !strings.Contains(liste2, "vide") {
		t.Errorf("liste après retrait, le vide n'est pas nommé :\n%s", liste2)
	}
}

// TestEchangeFichiersCleRefuseeCotePair : la clé du voisin est ce que le pont exige. Un voisin
// déclaré avec la mauvaise clé doit être nommé sur la CLÉ, pas sur le fichier.
func TestEchangeFichiersCleRefuseeCotePair(t *testing.T) {
	parallelTestHome(t)
	echangeTestCle(t)

	bridgeLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skip("aucun port libre pour ce test")
	}
	pontB := &http.Server{Handler: parallelBridgeHandler()}
	go func() { _ = pontB.Serve(bridgeLn) }()
	defer pontB.Close()

	p, err := peerAddDeclared("B", "http://127.0.0.1:"+strconv.Itoa(bridgeLn.Addr().(*net.TCPAddr).Port)+"/v1", "pas-la-bonne-cle", "")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := peerFichiersBornes(context.Background())
	defer cancel()
	if _, err := peerListeFichiers(ctx, p); err == nil {
		t.Fatal("liste acceptée avec la mauvaise clé")
	} else if !strings.Contains(err.Error(), "clé") {
		t.Errorf("refus de clé mal nommé : %v", err)
	}
}
