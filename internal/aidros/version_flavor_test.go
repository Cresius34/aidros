package aidros

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"golang.org/x/mod/semver"
)

// TestVersionFlavorWindowsSuit : la ressource Windows du build portable
// (cmd/ajean-memory-test/versioninfo.json) doit annoncer exactement la même
// version que displayVersion(). Le lanceur du TEST (Lancer-Test.ps1) compare
// ProductVersion à la valeur attendue ; une divergence rend le paquet refusé,
// comme ce fut le cas en 0.8.0 (constante à jour, ressource ancienne).
func TestVersionFlavorWindowsSuit(t *testing.T) {
	BuildFlavor = "embedding-test"
	t.Cleanup(func() { BuildFlavor = "" })
	want := displayVersion()

	b, err := os.ReadFile("../../cmd/ajean-memory-test/versioninfo.json")
	if err != nil {
		t.Fatal(err)
	}
	var vi struct {
		StringFileInfo struct{ FileVersion, ProductVersion string }
	}
	if err = json.Unmarshal(b, &vi); err != nil {
		t.Fatal(err)
	}
	for name, got := range map[string]string{
		"StringFileInfo.ProductVersion": vi.StringFileInfo.ProductVersion,
		"StringFileInfo.FileVersion":    vi.StringFileInfo.FileVersion,
	} {
		if got != want {
			t.Errorf("%s = %q, attendu %q — pense à `go generate ./cmd/ajean-memory-test` après un bump", name, got, want)
		}
	}
	if want != Version+buildFlavorSuffix {
		t.Errorf("displayVersion() = %q, attendu %q", want, Version+buildFlavorSuffix)
	}
	// LA GARDE PORTE SUR displayVersion(), plus sur la longueur du suffixe.
	//
	// Depuis la campagne du 24/11, le millésime est la base à TROIS chiffres
	// ("1.1.3") et le suffixe est vide : l'ancienne garde
	// (« len(suffixe) >= 2 et premier caractère '.' ou '+' ») serait ROUGE sur un
	// millésime parfaitement juste — un garde rouge sur du juste est aussi
	// inutile qu'un garde vert sur du faux.
	//
	// Ce qui compte, mesuré dans sys_update.go:190, c'est ce que le poste
	// AFFICHE et ce que le comparateur lit : un millésime à trois chiffres,
	// comparable, sans « + ».
	//   · quatre chiffres (« 1.1.3.3 », ce que rend "1.1.3" + ".3") :
	//     semver.IsValid refuse, sys_update.go:185 rend « tag de release
	//     inattendu » et il ne se propose plus JAMAIS rien, silencieusement ;
	//   · deux chiffres ("1.1") : il canonise en « v1.1.0 », donc publier v1.1.3
	//     laisse Compare > 0 pour toujours — la mise à jour s'offre en boucle ;
	//   · un « + » : semver.Canonical l'efface, la release devient invisible
	//     (mesuré 22/11, le défaut de v1.0.0+rev0).
	if !semver.IsValid(ensureV(want)) {
		t.Errorf("displayVersion() = %q : semver.IsValid est faux, la mise à jour ne pourra jamais comparer ce millésime", want)
	}
	if n := len(strings.Split(want, ".")); n != 3 {
		t.Errorf("displayVersion() = %q : %d chiffres, attendu 3. Quatre = semver invalide (la mise à jour se tait) ; deux = canonisé en x.y.0 (la mise à jour se propose en boucle)", want, n)
	}
	if strings.Contains(want, "+") {
		t.Errorf("displayVersion() = %q porte un « + » : semver.Canonical l'efface, la release serait invisible à l'auto-maj (défaut de v1.0.0+rev0, mesuré 22/11)", want)
	}
	// Si un suffixe revient, il reste un incrément de correctif en « . », jamais
	// un « + » : c'est la forme qu'admet le test de version Windows, et celle que
	// le lanceur d'un poste sait lire.
	//
	// HasPrefix et non suffixe[0] : le suffixe EST vide aujourd'hui, et vet l'a
	// signalé (« invalid argument: index 0 out of bounds ») — le court-circuit
	// m'aurait sauvé à l'exécution, la constante connue vide ne pardonne pas.
	if buildFlavorSuffix != "" &&
		!strings.HasPrefix(buildFlavorSuffix, ".") &&
		!strings.HasPrefix(buildFlavorSuffix, "+") {
		t.Errorf("buildFlavorSuffix = %q, doit commencer par '+' ou '.' (portable, sans '+')", buildFlavorSuffix)
	}
}

// TestVersionFlavorPlainEntry : le point d'entrée normal (cmd/aidros) n'ajoute
// aucun suffixe : Version seule.
func TestVersionFlavorPlainEntry(t *testing.T) {
	BuildFlavor = ""
	if got := displayVersion(); got != Version {
		t.Errorf("displayVersion() sans flavor = %q, attendu %q", got, Version)
	}
}
