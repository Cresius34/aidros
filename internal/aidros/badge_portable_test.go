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
