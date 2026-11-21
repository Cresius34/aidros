package aidros

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"golang.org/x/mod/semver"
)

// La ressource de version Windows (cmd/aidros/versioninfo.json) doit suivre la
// constante Version. Elles ont divergé en 0.8.0 : la constante était à jour, la
// ressource restait à 0.7.13. Conséquence peu lisible — le binaire installé se
// DÉCLARAIT 0.7.13 auprès de Windows, si bien que relancer le fichier téléchargé
// proposait indéfiniment « appliquer la mise à jour ? » sur une version déjà
// installée. Ce test rend l'oubli impossible.
func TestVersionInfoWindowsSuitLaVersion(t *testing.T) {
	b, err := os.ReadFile("../../cmd/aidros/versioninfo.json")
	if err != nil {
		t.Fatal(err)
	}
	var vi struct {
		FixedFileInfo struct {
			FileVersion    struct{ Major, Minor, Patch int }
			ProductVersion struct{ Major, Minor, Patch int }
		}
		StringFileInfo struct{ ProductVersion string }
	}
	if err := json.Unmarshal(b, &vi); err != nil {
		t.Fatal(err)
	}
	f := vi.FixedFileInfo.FileVersion
	// ⚠ La comparaison passe par semver.Canonical, pas par l'égalité des chaînes.
	// Depuis la scission actée le 21/11, notre millésime est la base à DEUX
	// chiffres ("1.0") parce que buildFlavorSuffix y ajoute le cran de correctif
	// ("1.0" + ".0" = « 1.0.0 »). Or FixedFileInfo ne sait écrire que trois
	// nombres : il rend "1.0.0" là où Version dit "1.0". Comparer les chaînes
	// rendrait le garde ROUGE sur une version juste — et un garde rouge sur du
	// juste est aussi inutile qu'un garde vert sur du faux.
	//
	// Canonical est la fonction qu'il faut, et c'est la doc du module qui le
	// dit, pas mon souvenir (« x/mod/semver@v0.35.0 », semver.go, en-tête du
	// paquet) : « it recognizes vMAJOR and vMAJOR.MINOR (with no prerelease or
	// build suffixes) as shorthands for vMAJOR.0.0 and vMAJOR.MINOR.0 ». Donc
	// Canonical("v1.0") == Canonical("v1.0.0") == "v1.0.0".
	//
	// Ne pas confondre avec sys_update.go, qui lui utilise IsValid + Compare
	// (comparer deux versions), ni avec un « Normalize » que je n'ai pas
	// vérifié et qui n'existe pas : vet l'a refusé, et il a eu raison.
	constVersion := semver.Canonical(ensureV(Version))
	if constVersion == "" {
		t.Fatalf("Version = %q : semver.Canonical rend vide, la constante n'est pas un millésime exploitable", Version)
	}
	for name, got := range map[string]string{
		"FixedFileInfo.FileVersion":     fmt.Sprintf("%d.%d.%d", f.Major, f.Minor, f.Patch),
		"FixedFileInfo.ProductVersion":  fmt.Sprintf("%d.%d.%d", vi.FixedFileInfo.ProductVersion.Major, vi.FixedFileInfo.ProductVersion.Minor, vi.FixedFileInfo.ProductVersion.Patch),
		"StringFileInfo.ProductVersion": vi.StringFileInfo.ProductVersion,
	} {
		if semver.Canonical(ensureV(got)) != constVersion {
			t.Errorf("%s = %s, attendu %s (sous sa forme canonique %s) — pense à `go generate ./cmd/aidros` après un bump", name, got, Version, constVersion)
		}
	}
}
