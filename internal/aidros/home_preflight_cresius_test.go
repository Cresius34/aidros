package aidros

// TestPreflightHomeCresius — la preuve exécutée du defaut rencontre par
// l'humain le 14/11/2026 a 22:4x : le binaire 0.20.1.1 Cresius archive dans
// archive-0.20.1.1-veilleur-20261114\ ne demarre PAS Strata quand on selectionne
// un preset MoE.
//
// moeStrataPreflight resout MOE_CONFIG / MOE_SOURCE / MOE_PYTHON RELATIFS depuis
// AjeanHome() (backend_moe_strata.go:145 + strataPath). AjeanHome() n'a que trois
// sources (run.go:228) : $AJEAN_HOME, /etc/default/ajean (absent sous Windows),
// puis defaultAjeanHome() = homeRoot()+"\Aidros" = C:\ProgramData\Aidros (le nom
// « ajean » d'avant reste admis s'il existe déjà, voir repli_dossier_aidros_test.go).
//
// Mesure de ce soir : AJEAN_HOME n'est dans le registre NI machine NI utilisateur.
// L'app lancee depuis l'explorateur (chaine des peres mesuree : explorer.exe ->
// retour-binaire-...-ajean.exe) ne recoit donc PAS la variable. Le home tombe au
// repli, ou il n'y a aucun data\moe : le preflight rend « installation MoE
// incomplete », ce qui est exactement « ne lance pas le moteur ».
//
// Le test ne demarre rien : moeStrataPreflight ne fait que stat() et lire
// BUILD.json. Lecture seule, aucun moteur de 74 Go, aucun reseau.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// les cles telles que les presets les portent : relatives, resolues depuis le home.
func cresiusMoeConfig() map[string]string {
	return map[string]string{
		"ENGINE":      "moe",
		"MOE_BACKEND": "hip",
		"MOE_CONFIG":  filepath.Join("moe", "run", "strata-hip.json"),
		"MOE_SOURCE":  filepath.Join("moe", "strata-"+strataVersion+"-hip"),
		"MOE_PYTHON":  filepath.Join("moe", "strata-"+strataVersion+"-hip", "python", "python.exe"),
		"MOE_VISION":  "0",
	}
}

// TestPreflightSurLeVraiHome : ce que l'app trouverait si AJEAN_HOME est bien
// heritee. Le moteur est installe, donc la seule chose qui puisse manquer est
// l'appairage engine/BUILD.json avec la constante epinglee par ce binaire.
func TestPreflightSurLeVraiHome(t *testing.T) {
	reel := `C:\ProgramData\Ajean v20.1 maj Cresius\data`
	if _, err := os.Stat(filepath.Join(reel, "moe", "strata-"+strataVersion+"-hip")); err != nil {
		t.Skip("le moteur " + strataVersion + " n'est pas installe la, rien a mesurer")
	}
	t.Setenv("AJEAN_HOME", reel)
	_, base, err := moeStrataPreflight(cresiusMoeConfig())
	if err == nil {
		t.Logf("home reel : PREFLIGHT OK, moteur %v backend %v", base["version"], base["backend"])
		return
	}
	// Le refus attendu, nomme pour qu'on ne le confonde plus jamais avec l'autre.
	t.Logf("home reel : refuse — %v", err)
	if !strings.Contains(err.Error(), "version fig") {
		t.Fatalf("home reel : une piece manque, ce n'est pas l'appairage : %v", err)
	}
}

// TestPreflightSurLeRepli : ce que l'app trouve quand AJEAN_HOME n'est pas
// heritee, c'est-a-dire quand on la lance en double-cliquant. C'est LA cause du
// « le binaire archive ne demarre pas Strata » : l'archive n'a aucun data a cote
// d'elle, et le dossier de repli n'a aucun moteur installe.
//
// Le repli est resolu PAR LA FONCTION, pas ecrit en dur : il s'appelle « Aidros »
// depuis le 21/11 et « ajean » avant, et un test qui forcerait l'un des deux noms
// cesserait de tester le repli le jour ou le nom bouge — sans le dire.
//
// DEUX REFUS MESURES, et pas un. Sonde jouee le 22/11/2026 sous Windows, et
// journal du run #5 de la CI (sha 4185387) sous ubuntu a l'appui :
//
//	home EXISTANT et sans data\moe (la machine de l'humain) →
//		« Python ou serveur MoE absent : reprendre l'installation depuis Moteur »,
//		par le garde isFile(py) || !isFile(src\serve\server.py) de
//		backend_moe_strata.go:159.
//	home INEXISTANT (le repli d'un runner, /etc/aidros) →
//		un « lstat …: no such file or directory » NU, rendu par
//		filepath.EvalSymlinks(root) de parallelPathUnder (parallel_config.go:240),
//		qui n'a aucune tolerance pour un home absent : la boucle « missing » en
//		dessous ne traite que les CIBLES manquantes, jamais la racine.
//
// Les deux disent la MEME chose — ici, il n'y a pas de moteur — donc la
// demonstration tient sur les deux. Ce qui est ecarte NOMMEMENT, pour ne jamais
// confondre : « backend MoE invalide » et « installation MoE incomplete » (cles
// MOE_* vides ou fausses : le preset est mal ecrit, ce n'est pas un repli vide)
// et « version figee » (l'appairage engine/BUILD.json, l'autre defaut de la
// série Gyro). Un home inexistant n'est pas un preset mal ecrit.
func TestPreflightSurLeRepli(t *testing.T) {
	repli := defaultAjeanHome()
	t.Setenv("AJEAN_HOME", repli)
	if _, err := os.Stat(filepath.Join(repli, "moe")); err == nil {
		t.Skip("le repli a un data\\moe : la demonstration ne tient plus ici")
	}
	_, _, err := moeStrataPreflight(cresiusMoeConfig())
	if err == nil {
		t.Fatal("le repli a passe le preflight : la demonstration est fausse")
	}
	t.Logf("repli %s : refuse — %v", repli, err)
	msg := err.Error()
	trou := strings.Contains(msg, "MoE absent") || strings.Contains(msg, "no such file")
	if !trou {
		t.Fatalf("message inattendu : %v", err)
	}
	for _, autre := range []string{"incomplet", "version fig", "backend MoE invalide"} {
		if strings.Contains(msg, autre) {
			t.Fatalf("ce n'est plus la demonstration du repli vide, c'est « %s » : %v", autre, err)
		}
	}
	// Le message REEL sous Windows, mesure a l'instant : « Python ou serveur MoE
	// absent : reprendre l'installation depuis Moteur ». C'est le garde
	// isFile(py) || !isFile(src\serve\server.py) de backend_moe_strata.go:159 qui
	// parle, pas celui de l'installation vide : les cles MOE_* sont bien remplies
	// (elles sont relatives et resolues), ce sont les FICHIERS qui manquent au
	// repli. « installation MoE incomplete » serait rendu par des cles vides : ce
	// n'est pas le cas ici, et l'ecarter evite de le confondre avec l'autre.
}

// TestHomeDependDeLArchive : la regle que mon script de bascule doit respecter.
// Un binaire pose dans un sous-dossier n'a pas de data a cote de lui ; poser
// l'exe dans archive-* ne peut donc pas marcher, il faut l'echange RACINE.
func TestHomeDependDeLArchive(t *testing.T) {
	racine := `C:\ProgramData\Ajean v20.1 maj Cresius`
	for _, e := range []struct{ nom, exe string }{
		{"la racine, ce qui sert", filepath.Join(racine, "ajean.exe")},
		{"l'archive de sauvegarde", filepath.Join(racine, "archive-0.20.1.1-veilleur-20261114", "ajean.exe")},
	} {
		if _, err := os.Stat(e.exe); err != nil {
			t.Logf("%-24s : exe absent, rien a dire", e.nom)
			continue
		}
		cote := filepath.Join(filepath.Dir(e.exe), "data")
		_, err := os.Stat(cote)
		t.Logf("%-24s : data a cote de l'exe -> %v", e.nom, err == nil)
		if e.nom == "la racine, ce qui sert" && os.IsNotExist(err) {
			t.Fatal("la racine n'a plus de data : la machine a change, relire la mesure")
		}
	}
}
