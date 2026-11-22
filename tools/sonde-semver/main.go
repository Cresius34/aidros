package main

// Sonde : ce que semver rend pour NOS formes de version candidates, face a la
// derniere release de l'amont (v1.0.10). sys_update.go:163 fait exactement
// semver.Compare(ensureV(latest), ensureV(Version)) > 0 pour decider
// « mise a jour disponible ». Une chaine invalide compare MOINS qu'une valide.

import (
	"fmt"
	"strings"

	"golang.org/x/mod/semver"
)

func ensureV(s string) string {
	s = strings.TrimSpace(s)
	if s != "" && !strings.HasPrefix(s, "v") {
		s = "v" + s
	}
	return s
}

func main() {
	latest := ensureV("v1.0.10") // la release la plus recente de nathaninline/ajean
	fmt.Println("la release amont la plus récente :", latest, "| semver valide =", semver.IsValid(latest))
	fmt.Println()
	for _, c := range []string{
		"0.20.1",  // ce qu'on porte aujourd'hui
		"1.00",    // la forme litterale de l'ordre
		"1.0.0",   // la forme semver
		"1.0.0.1", // avec un cran de revision
		"1.0.0+rev0", "1.0.0+rev0 Cresius", "1.0.0.0 Cresius", "1.0.0+rev0 Cresius",
	} {
		v := ensureV(c)
		valide := semver.IsValid(v)
		dispo := semver.Compare(latest, v) > 0
		fmt.Printf("Version = %-13s -> %-16s valide=%-5v  l'amont est « disponible » = %v\n",
			c, v, valide, dispo)
	}
	fmt.Println()
	// Et avec le suffixe Cresius colle a la version (displayVersion) :
	for _, c := range []string{"1.0.0 Cresius", "1.0.0.1 Cresius"} {
		v := ensureV(c)
		fmt.Printf("displayVersion = %-18s -> %-24s valide=%-5v\n", c, v, semver.IsValid(v))
	}

	sondeDizaine()
}

// sondeDizaine : ce qui a ete demande le 22/11, « quid si on arrive a la
// dizaine ? ». Deux questions distinctes, joueées separement parce qu'elles
// n'ont rien a voir l'une avec l'autre :
//
//	A. la DIZAINE NUMERIQUE est-elle tenue par semver ? (la reponse est oui, et
//	   le zero colle v1.0.010 est refuse : c'est ce que l'humain prefere eviter)
//	B. le +revN est-il VU par le comparateur ? (la reponse est non : la
//	   metadata de build ne compte pas dans le classement semver. Une release
//	   etiquetee +revN est donc publiee et jamais proposee a personne.)
func sondeDizaine() {
	fmt.Println()
	fmt.Println("=== A. LA DIZAINE NUMERIQUE, forme par forme ===")
	for _, c := range []string{
		"v1.0.8", "v1.0.9", "v1.0.10", "v1.0.11", // la dizaine, dans l'ordre
		"v1.0.010", "v1.0.009", // le zero colle : la forme a eviter
		"v1.0.100", "v1.10.0", "v1.100.0", // la centaine, et le nombre a deux chiffres
	} {
		fmt.Printf("  %-12s valide=%-5v  Canonical=%q\n", c, semver.IsValid(c), semver.Canonical(c))
	}
	fmt.Println("  le compare-t-il dans l'ordre croissant ?")
	ordre := []string{"v1.0.8", "v1.0.9", "v1.0.10", "v1.0.11", "v1.1.0"}
	for i := 0; i+1 < len(ordre); i++ {
		fmt.Printf("    %-9s < %-9s ? %v\n", ordre[i], ordre[i+1], semver.Compare(ordre[i], ordre[i+1]) < 0)
	}

	fmt.Println()
	fmt.Println("=== B. ce que +revN devient aux yeux du comparateur ===")
	fmt.Println("    sys_update.go:185 ne compare QUE le millésime sans la metadata :")
	for _, c := range []string{"v1.0.0+rev0", "v1.0.0+rev1", "v1.0.0+rev10", "v1.0.0+rev99", "v1.0.0+rev100"} {
		fmt.Printf("  %-16s valide=%-5v  Canonical=%q\n", c, semver.IsValid(c), semver.Canonical(c))
	}
	fmt.Printf("  v1.0.0+rev1  vs v1.0.0+rev10  Compare = %d  (0 = la release d'apres est invisible)\n",
		semver.Compare("v1.0.0+rev1", "v1.0.0+rev10"))

	fmt.Println()
	fmt.Println("=== C. ce que voit la machine qui tourne, selon sa propre forme ===")
	// Deux formes tournent selon l'entrée appelée : cmd/aidros publie la Version
	// nue (« 1.0 », run.go:22), le build portable y ajoute buildFlavorSuffix
	// (« .0 » → « 1.0.0 », parallel_profile.go:84).
	for _, tourne := range []string{"1.0", "1.0.0"} {
		fmt.Printf("  une machine tourne %s :\n", tourne)
		for _, c := range []string{"v1.0.0", "v1.0.0+rev0", "v1.0.1", "v1.0.11", "v1.1.0"} {
			cmp := semver.Compare(c, ensureV(tourne))
			mot := "rien de propose"
			if cmp > 0 {
				mot = "MISE A JOUR proposee"
			}
			if !semver.IsValid(c) {
				mot = "ETIQUETTE REFOUSÉE"
			}
			fmt.Printf("    %-14s Compare = %-3d -> %s\n", c, cmp, mot)
		}
	}
}
