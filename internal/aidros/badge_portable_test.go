package aidros

// Le badge du build portable ne doit pas répéter ce que le logo dit déjà.
//
// Mesuré le 22/11 sur l'interface de la capture de l'humain : sous le logo, la
// ligne « #ver » affiche le millésime (05-status.js:62, lu sur /api/status) et le
// mot-pixel épelette Aidros. Le badge y ajoutait une troisième ligne écrivant
// « Aidros » en gras puis « 1.0.0 ». Le nom et le millésime, deux fois, à deux
// tailles, à trois pixels l'un de l'autre.
//
// Ce fichier est le SEUL endroit où cette promesse est tenue. Sans lui, le
// doublon peut revenir ligne à ligne sans qu'aucun filet ne bronche : le garde de
// livraison (parallel_delivery_test.go:20) exige displayVersion() et uiBadgeLabel
// dans la page, et les trouve dans le <title> et l'infobulle de l'icône — il ne
// regarde pas le badge.

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// avecBuildPortable pose le build portable pour la durée du test, et rend sa
// valeur d'origine ensuite. Le posage est explicite, et non hérité : un autre
// fichier de test touche déjà BuildFlavor (parallel_delivery_test.go:30 le vide),
// donc la valeur au moment où celui-ci tourne dépend de l'ordre des tests.
func avecBuildPortable(t *testing.T) {
	t.Helper()
	avant := BuildFlavor
	BuildFlavor = "embedding-test"
	t.Cleanup(func() { BuildFlavor = avant })
}

// TestBadgePortableSansDoublon : le nom du test est en ASCII pur, sans accent.
// Un -run joué depuis un .bat est lu par cmd.exe dans la page de codes de la
// console : les é de mes noms de tests avaient déjà cassé un .bat (mesuré
// 21/11), et un -run sans accent ne sélectionne rien — un test non joué rend un
// vert qui ne vérifie rien. Nommer en ASCII ferme les deux pièges.
func TestBadgePortableSansDoublon(t *testing.T) {
	parallelTestHome(t)
	avecBuildPortable(t)

	// Le port et le dossier sont forcés : ce sont eux que le badge doit porter, et
	// les laisser venir de la base ou de l'environnement rendrait le test vert pour
	// une valeur qu'il n'a pas choisie.
	t.Setenv("AJEAN_UI_PORT", "18099")
	dossier := filepath.Join(t.TempDir(), "mon-poste", "data")
	if err := os.MkdirAll(dossier, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AJEAN_HOME", dossier)

	w := httptest.NewRecorder()
	handleIndex(w, httptest.NewRequest("GET", "http://127.0.0.1/", nil))
	if w.Code != 200 {
		t.Fatalf("page non servie : %d", w.Code)
	}
	body := w.Body.String()

	// 1. Le badge est bien posé : la SIGNATURE, pas le mot.
	i := strings.Index(body, uiBadgeMarker)
	if i < 0 {
		t.Fatal("badge absent de la page servie")
	}
	fin := strings.Index(body[i:], "</div>")
	if fin < 0 {
		t.Fatalf("badge non referme : %s", body[i:i+200])
	}
	badge := body[i : i+fin]

	// 2. ⭐ Le nom et le millésime n'y sont PLUS. C'est le doublon que l'humain a
	// montré du doigt : le logo épelette déjà Aidros, « #ver » affiche déjà le
	// millésime.
	//
	// ⚠ La casse fait toute la différence ici, et ce n'est pas un hasard à
	// exploiter sans le dire : depuis le 22/11 le badge porte l'URL du dépôt,
	// « https://github.com/Cresius34/aidros », qui contient le nom du produit en
	// MINUSCULES. Ce test compare en casse exacte, il reste vert. Ce qu'il défend
	// est le doublon d'ÉTIQUETTE — le badge qui écrit « Aidros » en gras sous un
	// logo qui l'épelait déjà — et une adresse de dépôt n'est pas une étiquette :
	// on ne lit pas l'URL d'un dépôt pour savoir comment le logiciel s'appelle.
	// Renommer le dépôt un jour ne renommera pas le produit.
	if strings.Contains(badge, uiBadgeLabel) {
		t.Errorf("le badge répète le nom du produit (%q) alors que le logo l'affiche déjà : %s", uiBadgeLabel, badge)
	}
	if strings.Contains(badge, displayVersion()) {
		t.Errorf("le badge répète le millésime (%q) alors que « #ver » l'affiche déjà : %s", displayVersion(), badge)
	}

	// 3. Ce que le badge dit, lui seul le dit : l'interface et l'instance.
	if !strings.Contains(badge, "18099") {
		t.Errorf("le badge ne nomme pas l'interface d'écoute : %s", badge)
	}
	if !strings.Contains(badge, "mon-poste") {
		t.Errorf("le badge ne nomme pas l'instance : %s", badge)
	}

	// 4. Le millésime reste exposable dans la page, mais à sa place : le <title>.
	// Le garde de livraison l'y exige (parallel_delivery_test.go:20) — ici on nomme
	// l'endroit, pour qu'enlever le badge ne l'enlève pas de la page.
	if !strings.Contains(body, "<title>Aidros "+displayVersion()) {
		t.Errorf("le millésime a disparu du <title> : %s", body[:200])
	}

	// 5. Le point d'entrée normal ne connaît toujours pas ce badge.
	BuildFlavor = ""
	w = httptest.NewRecorder()
	handleIndex(w, httptest.NewRequest("GET", "http://127.0.0.1/", nil))
	if strings.Contains(w.Body.String(), uiBadgeMarker) {
		t.Error("badge du patch affiché dans le point d'entrée normal")
	}
}

// Le nom du produit ne disparaît pas de l'interface pour avoir quitté le badge,
// et ce n'est PAS ce fichier qui en répond : il vit sur l'infobulle de l'icône
// système (libellesTray, sys_marque_os.go:42), déjà gardé par
// marque_os_test.go:92, et dans le <title> de la page — vérifié au test 4
// ci-dessus. Ne pas redire ici ce qu'un autre filet tient déjà.

// TestBadgePortableLieLeDepot : la ligne sous le logo porte un lien vers NOTRE
// dépôt, dans les DEUX langues. Posé le 22/11 sur la demande de l'humain, qui
// montrait cette ligne du doigt.
//
// Cinq choses sont vérifiées, et pas une seule :
//
//  1. L'URL vient de updateRepo. C'est le point qui mord : écrire l'adresse en
//     dur dans le badge ferait DEUX vérités sur l'emplacement du dépôt. L'histoire
//     de cette URL est assez parlante — portée à « Cresius34/aidros » le 21/11,
//     elle faisait un 404 sur /api/update tant que la release n'existait pas, et
//     le 404 vu chez la voisine venait d'un exe portant l'URL ARCHIVÉE
//     Cresius34/ajean, c'est-à-dire d'une adresse figée dans le marbre d'un
//     binaire. Un second endroit à mettre à jour à la main est un second endroit
//     qui désynchronisera.
//
//  2. Le lien s'ouvre dans un onglet isolé.
//
//  3. Le libellé est écrit dans la page en ANGLAIS, la langue par défaut
//     (00b-i18n.js:20, currentLang replie sur « en ») : c'est ce que le serveur
//     sert. Si le HTML portait le français, le navigateur le remplacerait à
//     l'anglais et le texte clignoterait.
//
//  4. La CLÉ existe en français comme en anglais, et les deux textes DIFFÈRENT.
//     Le texte français ne peut pas être prouvé par la page servie — le moteur le
//     pose à l'exécution, dans le navigateur. Ce qui se vérifie côté serveur,
//     c'est la traduction : le filet lit le dictionnaire.
//
//  5. Le lien n'évince pas ce que la ligne disait déjà, et le point d'entrée
//     normal reste étranger au badge.
//
// Le nom du test est en ASCII pur, comme TestBadgePortableSansDoublon ci-dessus :
// un -run accentué joué depuis un .bat ne sélectionne rien, et un test non joué
// rend un vert qui ne vérifie rien.
func TestBadgePortableLieLeDepot(t *testing.T) {
	parallelTestHome(t)
	avecBuildPortable(t)

	// Port et dossier forcés, comme dans l'autre test : ce sont eux que le badge
	// doit porter, et les laisser venir de l'environnement rendrait le test vert
	// pour une valeur qu'il n'a pas choisie.
	t.Setenv("AJEAN_UI_PORT", "18098")
	dossier := filepath.Join(t.TempDir(), "poste-lien", "data")
	if err := os.MkdirAll(dossier, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AJEAN_HOME", dossier)

	w := httptest.NewRecorder()
	handleIndex(w, httptest.NewRequest("GET", "http://127.0.0.1/", nil))
	if w.Code != 200 {
		t.Fatalf("page non servie : %d", w.Code)
	}
	body := w.Body.String()

	i := strings.Index(body, uiBadgeMarker)
	if i < 0 {
		t.Fatal("badge absent de la page servie")
	}
	fin := strings.Index(body[i:], "</div>")
	if fin < 0 {
		t.Fatalf("badge non referme : %s", body[i:i+200])
	}
	badge := body[i : i+fin]

	// 1. L'URL du dépôt, ASSEMBLÉE depuis sa source unique.
	if !strings.Contains(badge, "https://github.com/"+updateRepo) {
		t.Errorf("le badge ne mène pas au dépôt (%q) : %s", updateRepo, badge)
	}
	// Et le dépôt archivé n'y revient pas : c'est l'adresse qui faisait le 404.
	if strings.Contains(badge, "Cresius34/ajean") {
		t.Errorf("le badge pointe le dépôt archivé : %s", badge)
	}

	// 2. Sans rel="noopener", la page du dépôt tient la main sur la fenêtre qui
	// l'a ouverte. Le gabarit fait déjà ainsi pour son autre lien externe
	// (index.tmpl.html:1048).
	if !strings.Contains(badge, `target="_blank"`) || !strings.Contains(badge, `rel="noopener"`) {
		t.Errorf("le lien du dépôt n'ouvre pas un onglet isolé : %s", badge)
	}

	// 3 et 4. Les deux langues, lues dans le dictionnaire.
	d := libelleRepo(t)
	for _, langue := range []string{"fr", "en"} {
		if d[langue] == "" {
			t.Errorf("la clé sidebar.repo_link manque en %s", langue)
		}
	}
	if d["fr"] != "" && d["fr"] == d["en"] {
		t.Errorf("le libellé n'est pas traduit, identique dans les deux langues : %q", d["fr"])
	}
	if d["en"] != "" && !strings.Contains(badge, ">"+d["en"]+"<") {
		t.Errorf("le badge n'écrit pas le libellé de la langue par défaut (%q) : %s", d["en"], badge)
	}
	// La clé doit être posée sur l'élément, sinon applyI18n ne la touchera jamais.
	if !strings.Contains(badge, `data-i18n="sidebar.repo_link"`) {
		t.Errorf("le lien ne porte pas sa clé de traduction : %s", badge)
	}

	// 5a. L'interface d'écoute et l'instance sont toujours nommées.
	if !strings.Contains(badge, "18098") || !strings.Contains(badge, "poste-lien") {
		t.Errorf("le lien a évincé l'interface ou l'instance : %s", badge)
	}

	// 5b. Le point d'entrée normal ne connaît toujours pas ce badge.
	BuildFlavor = ""
	w = httptest.NewRecorder()
	handleIndex(w, httptest.NewRequest("GET", "http://127.0.0.1/", nil))
	if strings.Contains(w.Body.String(), uiBadgeMarker) {
		t.Error("badge du patch affiché dans le point d'entrée normal")
	}
}

// libelleRepo relève le texte de « sidebar.repo_link » dans chaque langue, en
// lisant le DICTIONNAIRE du dépôt et non la page servie : le serveur embarque le
// HTML dans sa langue par défaut, le français est posé par le navigateur.
//
// Le fichier est lu depuis le dossier courant du paquet, là où « go test » place
// le processus, et avec l'UTF-8 — jamais la page de codes de la console.
func libelleRepo(t *testing.T) map[string]string {
	t.Helper()
	ch, err := os.ReadFile("ui/src/js/00a-i18n-data.js")
	if err != nil {
		t.Fatalf("dictionnaire de traduction illisible : %v", err)
	}
	texte := string(ch)
	out := map[string]string{}
	for _, langue := range []string{"fr", "en"} {
		// Le bloc de langue, puis la clé à l'intérieur de ce bloc. Chercher la clé
		// dans tout le fichier rendrait toujours la première trouvée, donc
		// toujours le français : les deux langues ne seraient jamais comparées.
		ou := strings.Index(texte, "\n"+langue+": {")
		if ou < 0 {
			t.Fatalf("bloc de langue %q introuvable dans le dictionnaire", langue)
		}
		apres := texte[ou:]
		cle := `"sidebar.repo_link": "`
		k := strings.Index(apres, cle)
		if k < 0 {
			continue // la clé manque dans cette langue : le test le dira
		}
		depuis := apres[k+len(cle):]
		fin := strings.Index(depuis, `"`)
		if fin < 0 {
			continue
		}
		out[langue] = depuis[:fin]
	}
	return out
}
