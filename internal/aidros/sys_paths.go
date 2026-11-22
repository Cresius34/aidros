package aidros

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
)

// « Où sont mes fichiers ? » — la question que personne ne devrait avoir à poser.
// Un binaire unique qui s'auto-installe au premier lancement rend les
// emplacements invisibles : on ne sait plus si le .exe qu'on a double-cliqué EST
// l'application ou seulement un installateur, ni où atterrissent la config, les
// modèles et les fichiers que l'agent écrit. On centralise donc la réponse ici,
// et on l'expose à la fois en ligne de commande (`aidros where`) et dans l'UI.

type ajeanPaths struct {
	Home      string `json:"home"`      // AJEAN_HOME : racine des données
	Database  string `json:"database"`  // bddtchat.db : config, préférences, conversation, clés
	Exe       string `json:"exe"`       // binaire en cours d'exécution
	Installed string `json:"installed"` // binaire installé (peut différer de Exe)
	Workspace string `json:"workspace"` // dossier de travail du mode agent (jetable)
	Scripts   string `json:"scripts"`   // dossier des scripts de l'agent (protégé)
	Memory    string `json:"memory"`
	Presets   string `json:"presets"`
	Models    string `json:"models"`
	Backends  string `json:"backends"`
}

// nomProduit et nomVieuxProduit : le nom du binaire installé, aujourd'hui et
// hier. La marque à l'écran, les assets de release et le paquet Go portent déjà
// « aidros » ; le binaire installé était le dernier à dire « ajean ». Son nom
// d'hier reste nommé ici, et seulement ici : c'est le nom que porte le fichier
// sur la machine de celui qui a installé avant, donc un identifiant, pas la marque.
const (
	nomProduit      = "aidros"
	nomVieuxProduit = "ajean"
)

// exeName nomme le fichier exécutable pour un nom de produit : sous Windows le
// suffixe .exe est obligatoire pour que l'association et le PATH le reconnaissent.
func exeName(nom string) string {
	if runtime.GOOS == "windows" {
		return nom + ".exe"
	}
	return nom
}

// installedExePath est l'emplacement canonique du binaire après installation.
// Il diffère par plateforme : sous Unix les installateurs posent
// /usr/local/bin/aidros (c'est ce que référencent les unités systemd et le plist
// launchd) ; sous Windows, faute d'équivalent, on utilise AJEAN_HOME\bin ajouté
// au PATH utilisateur.
//
// Le nom suit celui du produit, déjà porté par le paquet Go (e956522) et par les
// assets de release (release.yml:106, qui s'appellent aidros-*). L'aide de
// run.go:165 invitait déjà à taper « aidros <commande> » : c'était la seule
// phrase du programme à contredire le fichier qu'il installe.
func installedExePath() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(binDir(), exeName(nomProduit))
	}
	return "/usr/local/bin/" + nomProduit
}

// aliasAvecCote nomme l'alias d'avant, posé À CÔTÉ du binaire qu'on lui donne —
// et non à côté du dossier d'installation global. Deux appelants, deux dossiers
// différents, et c'est voulu :
//   - l'installation entretient l'alias de la machine, à côté du binaire installé ;
//   - le ménage de mise à jour (sys_update.go) balaie les reliquats laissés à côté
//     du binaire QUI TOURNE, qui peut être ailleurs que dans binDir().
//
// Le nom d'avant est gardé comme ALIAS et non comme cible : une machine installée
// avant le renommage a son raccourci, son PATH et ses habitudes arrimés à ce nom.
// Le supprimer net la laisserait relancer un binaire périmé à chaque clic — le
// piège que remplaceEnCours décrit pour un alias laissé de côté.
//
// Le précédent est dans le dépôt : « jean.exe » y est devenu « ajean.exe »,
// l'ancien nom restant entretenu comme alias (voir retireVieuxBinaires).
func aliasAvecCote(canonique string) string {
	return filepath.Join(filepath.Dir(canonique), exeName(nomVieuxProduit))
}

func currentPaths() ajeanPaths {
	exe, _ := os.Executable()
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return ajeanPaths{
		Home:      AjeanHome(),
		Database:  dbPath(),
		Exe:       exe,
		Installed: installedExePath(),
		Workspace: agentWorkspace(),
		Scripts:   scriptsDir(),
		Memory:    memoryDir(),
		Presets:   presetsDir(),
		Models:    modelsDir(),
		Backends:  backendsDir(),
	}
}

func cmdWhere(args []string) error {
	p := currentPaths()
	fmt.Printf("Emplacements Aidros\n\n")
	for _, row := range [][2]string{
		{"données (AJEAN_HOME)", p.Home},
		{"base", p.Database},
		{"binaire en cours", p.Exe},
		{"binaire installé", p.Installed},
		{"travail de l'agent", p.Workspace},
		{"scripts", p.Scripts},
		{"mémoire", p.Memory},
		{"presets", p.Presets},
		{"modèles", p.Models},
		{"backends", p.Backends},
	} {
		fmt.Printf("  %-20s %s\n", row[0], row[1])
	}
	if p.Exe != p.Installed {
		fmt.Printf("\n%s tu exécutes une copie qui n'est PAS le binaire installé.\n", dim("[info]"))
		fmt.Printf("  Les mises à jour depuis l'application ne modifient que la copie lancée.\n")
	}
	return nil
}

// handlePaths (GET /api/paths) alimente le bloc « Emplacements » de l'UI.
func handlePaths(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, 200, currentPaths())
}
