package aidros

// L'URL d'ouverture du build portable doit nommer le PRODUIT, plus l'ancien
// usage.
//
// Mesuré le 22/11 : l'adresse ouverte dans le navigateur au démarrage s'appelait
// « ?memory-test=1.0.0 ». Elle datait de la preuve d'embeddings, l'époque où ce
// build servait à essayer un moteur de mémoire. Depuis, la même fenêtre s'intitule
// « Aidros » : la barre d'adresse contredisait le titre, comme le raccourci
// « AJEAN » contredisait son infobulle « Aidros » (corrigé le même jour, 8aca68b).
// Ordre de l'humain : « reformuler le memory-test de l'adresse en aidros-main ».
//
// Ce fichier est le SEUL endroit où cette promesse est tenue. Sans lui, le mot
// peut revenir sans qu'aucun filet ne bronche : le garde de livraison
// (parallel_delivery_test.go:20) regarde la PAGE servie, pas l'adresse qu'on
// ouvre dans le navigateur. Il ne voit donc jamais cette chaîne.
//
// ⚠ Les noms de tests sont en ASCII pur, sans accent. Un -run joué depuis un
// .bat est lu par cmd.exe dans la page de codes de la console : les é de mes
// noms de tests avaient déjà cassé un .bat (mesuré 21/11) — et un -run qui ne
// sélectionne rien rend un VERT QUI NE VÉRIFIE RIEN.

import (
	"net/url"
	"os"
	"strings"
	"testing"
)

// TestUrlOuvertureNommeLeProduit : ce que la barre d'adresse doit porter.
func TestUrlOuvertureNommeLeProduit(t *testing.T) {
	avecBuildPortable(t)

	const addr = "127.0.0.1:18099" // forcé : un filet qui lit un port par défaut serait faux ailleurs
	u := parallelOpenURL(addr)

	// 1. Le mot du produit, pas celui de l'ancien usage.
	if !strings.Contains(u, "/?"+uiOpenQueryParam+"=") {
		t.Errorf("l'URL ne nomme pas le produit (%q) : %s", uiOpenQueryParam, u)
	}
	for _, vieux := range []string{"memory-test=", "memoire-test=", "ajean=", "AJEAN"} {
		if strings.Contains(u, vieux) {
			t.Errorf("l'URL porte encore l'ancien nom %q : %s", vieux, u)
		}
	}

	// 2. Le millésime est toujours dedans, sous sa forme affichée. C'est le seul
	//    endroit où un humain lit « quelle version tourne » sans ouvrir l'interface.
	if !strings.Contains(u, "="+displayVersion()) {
		t.Errorf("l'URL ne porte plus le millésime (%q) : %s", displayVersion(), u)
	}

	// 3. L'hôte et le port viennent du PARAMÈTRE, pas d'un littéral. C'est
	//    l'adresse que net.Listen reçoit juste à côté, dans cmdParallelApp :
	//    écrire l'un des deux en dur ici rendait l'URL fausse le jour où l'autre
	//    bouge. Et 18090 n'est qu'un défaut (parallelUIPortDef), AJEAN_UI_PORT
	//    décale.
	if !strings.Contains(u, "//"+addr+"/") {
		t.Errorf("l'adresse demandée n'apparaît pas telle quelle : %s", u)
	}
	if strings.Contains(u, "18090") {
		t.Errorf("le port par défaut s'est glissé dans l'URL forcée : %s", u)
	}

	// 4. Ça reste une URL jouable : un seul point d'interrogation, un paramètre
	//    bien formé, et son nom lu tel quel par un parseur.
	parsed, err := url.Parse(u)
	if err != nil {
		t.Fatalf("ce n'est pas une URL : %v (%s)", err, u)
	}
	if got := parsed.Query().Get(uiOpenQueryParam); got != displayVersion() {
		t.Errorf("paramètre %q lu par un parseur = %q, attendu %q",
			uiOpenQueryParam, got, displayVersion())
	}
	if strings.Count(u, "?") != 1 {
		t.Errorf("%d signes « ? » dans l'URL, attendu 1 : %s", strings.Count(u, "?"), u)
	}
}

// TestUrlOuvertureDistingueDeuxInstallations : deux instances décalées doivent
// s'ouvrir sur des adresses différentes. Le port est la seule chose qui les
// sépare — si l'URL l'écrasait, deux onglets se ressembleraient à l'identique.
func TestUrlOuvertureDistingueDeuxInstallations(t *testing.T) {
	avecBuildPortable(t)

	une := parallelOpenURL("127.0.0.1:18090")
	autre := parallelOpenURL("127.0.0.1:18094")
	if une == autre {
		t.Errorf("deux ports différents rendent la même URL : %s", une)
	}
	if !strings.Contains(autre, "18094") {
		t.Errorf("le port décalé n'apparaît pas : %s", autre)
	}
}

// TestParametreOuvertureSansLecteurCache : le mot n'est PAS une porte, et un
// garde qui le croirait porteuse mentirait sur ce que le renommage coûte.
//
// Mesuré le 22/11 : r.URL.Query() n'est appelé nulle part sur l'interface, et le
// seul URLSearchParams du dépôt (13-remote.js:60) fabrique un paramètre
// origin/host, il ne lit pas celui-ci. Donc renommer n'efface AUCUNE logique.
// Si quelqu'un branche un lecteur sur ce mot, ce test rougit et la question de
// la compatibilité se repose — c'est l'usage d'un garde : dire le jour où ça change.
func TestParametreOuvertureSansLecteurCache(t *testing.T) {
	for _, f := range []string{
		"ui/src/index.tmpl.html",
		"ui/src/js/13-remote.js",
	} {
		// Chemin relatif au DOSSIER DU PAQUET : c'est de là que go test lance le
		// binaire de test, et c'est ce que fait déjà le voisin logo_marque_test.go:195.
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("%s : %v", f, err)
		}
		// ⚠ Le MOT NU ne suffit pas, mesuré par la contre-épreuve de ce fichier :
		// avec l'ancien nom, cette recherche trouvait « memory-test » dans
		// « memory-test-build » — l'IDENTIFIANT DOM DU BADGE, qui n'est pas le
		// paramètre d'URL et que je n'ai pas renommé. Deux surfaces distinctes,
		// chevauchées en sous-chaîne. Un garde ancré sur le mot nu rendait un
		// ROUGE qui ne reprochait rien, et m'aurait fait renommer le badge « pour
		// faire taire le filet » — exactement l'inverse du travail. Donc : la
		// FORME PARAMÈTRE, avec son signe égal.
		if strings.Contains(string(b), uiOpenQueryParam+"=") {
			t.Errorf("%s lit le paramètre d'ouverture : le renommage n'est plus décoratif, à trancher", f)
		}
	}
}
