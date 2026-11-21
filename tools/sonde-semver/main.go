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
}
