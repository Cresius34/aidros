package aidros

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
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
	// Deux raisons, toutes deux mesurées :
	//   · FixedFileInfo ne sait écrire que TROIS nombres (Major/Minor/Patch).
	//     Tant que le millésime était une base à deux chiffres ("1.0"), il rendait
	//     "1.0.0" là où Version disait "1.0" : comparer les chaînes rendait le
	//     garde ROUGE sur une version juste — et un garde rouge sur du juste est
	//     aussi inutile qu'un garde vert sur du faux.
	//   · Depuis la campagne du 24/11 le millésime EST à trois chiffres ("1.1.3")
	//     et la ressource le suit nombre par nombre. Canonical reste la bonne
	//     fonction : elle est neutre pour un millésime déjà complet, et elle est
	//     le seul moyen de rapprocher les deux formes.
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

// TestPublicationNeSeProposePasEnBoucle : la release que nous publions doit
// désigner EXACTEMENT le millésime que le binaire installé déclarera. Faute de
// quoi, mesuré dans sys_update.go:190
// (`semver.Compare(latest, ensureV(Version)) > 0`), l'application s'offre la
// MÊME mise à jour en boucle, sans jamais la terminer.
//
// Ce filet garde le défaut du 24/11, repéré la veille et nommé dans mes notes :
// publier v1.1.3 sur une Version "1.1" laissait Compare à +1 pour toujours,
// puisque "1.1" canonise en « v1.1.0 ». Aucun filet ne lisait la release
// publiée — c'était le trou.
//
// ⚠ Le binaire que publie le workflow est ./cmd/aidros (release.yml:115), où
// BuildFlavor n'est PAS posé : displayVersion() y rend Version SEULE, sans
// suffixe. C'est donc Version, et elle seule, que l'étiquette doit égaler.
func TestPublicationNeSeProposePasEnBoucle(t *testing.T) {
	BuildFlavor = ""
	t.Cleanup(func() { BuildFlavor = "" })
	installe := displayVersion()

	// 1. Le millésime est bien trois chiffres, comparable, sans « + ». Un « + »
	//    est effacé par Canonical : la release devient invisible (v1.0.0+rev0).
	if n := len(strings.Split(installe, ".")); n != 3 {
		t.Fatalf("displayVersion() = %q : %d chiffres. Trois chiffres = la seule forme où l'étiquette publiée et le binaire installé se rejoignent", installe, n)
	}
	if !semver.IsValid(ensureV(installe)) {
		t.Fatalf("displayVersion() = %q : semver.IsValid est faux, sys_update.go:185 rendrait « tag de release inattendu »", installe)
	}

	// 2. LE COEUR : l'étiquette de la release = « v » + ce que le binaire
	//    déclarera. Compare doit rendre 0 — sinon la mise à jour s'offre en
	//    boucle (positif) ou ne se propose jamais (négatif).
	etiquette := "v" + installe
	if c := semver.Compare(etiquette, ensureV(installe)); c != 0 {
		t.Errorf("Compare(%q, binaire installé %q) = %d, attendu 0 : %s",
			etiquette, installe, c,
			map[bool]string{
				true:  "la release est DÉJÀ dépassée par rapport au binaire — elle ne se propose pas",
				false: "la release se PROPOSERA EN BOUCLE : le binaire installé reste en dessous, il s'offre la même mise à jour sans jamais la terminer",
			}[c < 0])
	}

	// 3. Contre-épreuve, et la leçon du 23/11 figée dans le garde. Le comparateur
	//    bouge, et il bouge EXACTEMENT comme hier : une base à DEUX chiffres —
	//    « 1.1 », la mienne jusqu'à la campagne du 24/11 — canonise en « v1.1.0 »,
	//    donc Compare(v1.1.3, v1.1) rend +1 pour l'éternité. C'est ainsi qu'une
	//    release se propose en boucle sans qu'on y touche.
	//
	// ⚠ LE CAS HISTORIQUE EST ÉCRIT EN DUR, il n'est PAS DÉRIVÉ du millésime du
	//    jour, et la campagne 1.2.0 l'a prouvé en rougissant. Dérivé, le garde
	//    faisait Compare(installe, MajorMinor(installe)) : sur « 1.1.3 » il rendait
	//    +1 (le défaut), sur « 1.2.0 » il rend **0** — parce que « v1.2 » canonise
	//    en « v1.2.0 ». Autrement dit le garde ne prouvait rien dès que le
	//    correctif vaut 0 : il était satisfait par accident un jour sur dix.
	//    Une preuve qui dépend de ce qu'elle est censée prouver n'est pas une preuve.
	if c := semver.Compare("v1.1.3", "v1.1"); c != 1 {
		t.Errorf("Compare(v1.1.3, v1.1) = %d, attendu +1 : c'est le défaut du 23/11 figé dans le garde — la base nue se fait dépasser par sa propre release pour toujours", c)
	}

	// 3 bis. LE CAS DU JOUR, nommé pour ce qu'il est. Sur un millésime dont le
	//    correctif vaut 0, la base nue à deux chiffres RATTRAPE sa release :
	//    Compare(v1.2.0, v1.2) = 0. Ce n'est pas le défaut du 23/11, c'est
	//    l'inverse — Compare = 0 veut dire « rien n'est proposé », le binaire
	//    installé est bien la release. Écrit ici pour que le 0 ne soit plus jamais
	//    pris pour un rouge, et pour que le 23/11 ne soit plus jamais pris pour un
	//    vert : les deux dépendent du dernier chiffre, pas du comparateur.
	if c := semver.Compare("v1.2.0", "v1.2"); c != 0 {
		t.Errorf("Compare(v1.2.0, v1.2) = %d, attendu 0 : un millésime à correctif nul est ÉGAL à sa base nue, donc rien n'est proposé — c'est le comportement voulu, pas le défaut", c)
	}

	// 4. Autour de l'égalité, le comparateur rend bien 0 / +1 / −1.
	if c := semver.Compare(semver.MajorMinor(etiquette)+"."+strconv.Itoa(patchDe(etiquette)+1), ensureV(installe)); c != 1 {
		t.Errorf("la version de correctif suivante doit être proposée (+1), Compare = %d", c)
	}
	if p := patchDe(etiquette); p > 0 {
		prec := semver.MajorMinor(etiquette) + "." + strconv.Itoa(p-1)
		if c := semver.Compare(prec, ensureV(installe)); c != -1 {
			t.Errorf("%s doit être en dessous (−1), Compare = %d", prec, c)
		}
	}

	// 5. Le « + » ne doit jamais revenir dans une étiquette publiée.
	if strings.Contains(etiquette, "+") {
		t.Errorf("l'étiquette %q porte un « + » : Canonical l'efface, la release publiée serait invisible à l'auto-maj", etiquette)
	}
}

// patchDe renvoie le troisième nombre d'un millésime « vX.Y.Z ».
func patchDe(v string) int {
	p := strings.SplitN(strings.TrimPrefix(v, "v"), ".", 3)
	if len(p) < 3 {
		return 0
	}
	n := 0
	for _, c := range p[2] {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}
