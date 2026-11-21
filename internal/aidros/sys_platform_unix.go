//go:build unix

package aidros

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// defaultAjeanHome est la racine des données quand ni $AJEAN_HOME ni
// /etc/default/ajean n'en imposent une.
//
// Le dossier s'appelle « aidros » depuis le renommage du 21/11, le nom « ajean »
// restant ADMIS en second (voir sys_platform_windows.go : même contrat, même
// raison — une machine installée avant le renommé a sa mémoire sous l'ancien
// nom). Sur macOS, c'est ~/Library/Application Support/aidros.
//
// /etc/default/ajean garde son nom : c'est un fichier que l'installeur écrit et
// que lit readEtcDefault (run.go:249), sur une machine déjà installée. Le
// renommer sans lire l'ancien rendrait la racine introuvable chez l'utilisateur
// qui l'a. Il n'est donc PAS question d'y toucher ici.
func defaultAjeanHome() string { return unixHome("aidros", "ajean") }

// unixHome rend le chemin du dossier de données. Le nom d'aujourd'hui
// (nouveau) gagne ; le nom d'hier (ancien) n'est retenu QUE s'il existe déjà,
// parce qu'il y porte la mémoire et les clés de quelqu'un. Aucun test
// d'écriture n'est ajouté sur le chemin de l'admission : un simple Stat, pour ne
// pas aller poser une sonde dans /etc deux fois par appel.
func unixHome(nouveau, ancien string) string {
	// Linux, et macOS lancé par root : un chemin machine, sans test d'écriture.
	if runtime.GOOS != "darwin" || os.Geteuid() == 0 {
		if isDir("/etc/" + nouveau) {
			return "/etc/" + nouveau
		}
		if isDir("/etc/" + ancien) {
			return "/etc/" + ancien // la machine a été installée avant le renommé
		}
		return "/etc/" + nouveau // il sera créé
	}
	// macOS : /etc n'est PAS écrivable par une app lancée depuis le Finder
	// (« mkdir /etc/aidros: permission denied »), et un Mac est une machine de
	// bureau, pas un serveur — on retombe donc sur le dossier utilisateur.
	if isWritableDir("/etc/" + nouveau) {
		return "/etc/" + nouveau // installé par `sudo ajean install` puis chown à l'utilisateur
	}
	if home, err := os.UserHomeDir(); err == nil {
		for _, name := range []string{nouveau, ancien} {
			if cand := filepath.Join(home, "Library", "Application Support", name); isDir(cand) {
				return cand
			}
		}
		return filepath.Join(home, "Library", "Application Support", nouveau)
	}
	return "/etc/" + nouveau
}

// isWritableDir teste qu'un dossier existe ET qu'on peut y écrire (le seul test
// fiable : les permissions POSIX seules ignorent ACL, SIP, volumes read-only).
func isWritableDir(dir string) bool {
	fi, err := os.Stat(dir)
	if err != nil || !fi.IsDir() {
		return false
	}
	probe := filepath.Join(dir, ".ajean-write-test")
	f, err := os.OpenFile(probe, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return false
	}
	f.Close()
	_ = os.Remove(probe)
	return true
}

// defaultEditor is used by `ajean edit` when $EDITOR is unset.
func defaultEditor() string { return "nano" }

// hideCmd : no-op sur Unix (pas de fenêtre de console à masquer).
func hideCmd(cmd *exec.Cmd) *exec.Cmd { return cmd }

// openBrowser ouvre l'URL dans le navigateur par défaut (macOS: open, Linux:
// xdg-open). Best-effort.
func openBrowser(url string) error {
	bin := "xdg-open"
	if runtime.GOOS == "darwin" {
		bin = "open"
	}
	return exec.Command(bin, url).Start()
}

// totalRAMGB renvoie la RAM physique totale en Go (Linux: /proc/meminfo,
// macOS: sysctl hw.memsize).
func totalRAMGB() float64 {
	if runtime.GOOS == "darwin" {
		out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
		if err != nil {
			return 0
		}
		if v, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64); err == nil {
			return v / (1024 * 1024 * 1024)
		}
		return 0
	}
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			f := strings.Fields(line)
			if len(f) >= 2 {
				if kb, err := strconv.ParseFloat(f[1], 64); err == nil {
					return kb / (1024 * 1024) // kB → Go
				}
			}
		}
	}
	return 0
}

// setLibraryPath permet à llama-server de charger ses bibliothèques partagées :
// celles livrées à côté du binaire, celles de la variante GPU livrées avec le
// moteur (cudart, cublas), puis celles d'un toolkit CUDA/ROCm de la machine s'il
// y en a un.
// Sur macOS, dyld IGNORE LD_LIBRARY_PATH : la variable équivalente est
// DYLD_LIBRARY_PATH. Les binaires macOS des releases llama.cpp sont livrés avec
// leurs libllama/libggml*.dylib À CÔTÉ de l'exécutable — sans cette variable,
// llama-server meurt instantanément sur « Library not loaded », donc sans le
// moindre message dans l'UI.
func setLibraryPath(dir string) {
	envVar, joined := libraryPathValue(dir, engineLibDirsFor(dir))
	_ = os.Setenv(envVar, joined)
	if runtime.GOOS == "darwin" {
		// Filet de sécurité : DYLD_FALLBACK_LIBRARY_PATH est consulté en dernier
		// recours et survit à certains contextes où DYLD_LIBRARY_PATH est purgé.
		_ = os.Setenv("DYLD_FALLBACK_LIBRARY_PATH", joined+":/usr/local/lib:/usr/lib")
	}
}

// libraryPathValue calcule (nom de variable, valeur) sans toucher à
// l'environnement du process : utilisé pour lancer un llama-server ÉPHÉMÈRE
// (ex. `--list-devices` depuis l'UI) sans polluer — ni faire grossir à chaque
// appel — le LD_LIBRARY_PATH du serveur web lui-même.
func libraryPathValue(dir string, extra []string) (string, string) {
	parts := append([]string{dir}, extra...)
	parts = append(parts, gpuToolkitLibDirs()...)
	envVar := "LD_LIBRARY_PATH"
	if runtime.GOOS == "darwin" {
		envVar = "DYLD_LIBRARY_PATH"
	}
	if existing := os.Getenv(envVar); existing != "" {
		parts = append(parts, existing)
	}
	return envVar, strings.Join(parts, ":")
}

// libraryPathEnv renvoie un environnement complet (os.Environ + la variable de
// recherche de bibliothèques) pour une commande ponctuelle.
func libraryPathEnv(dir string) []string {
	return libraryPathEnvWith(dir, engineLibDirsFor(dir))
}

// libraryPathEnvWith : variante qui reçoit les dossiers de bibliothèques, pour
// sonder un moteur pas encore installé (voir engineProbe).
func libraryPathEnvWith(dir string, extra []string) []string {
	k, v := libraryPathValue(dir, extra)
	return append(os.Environ(), k+"="+v)
}

// gpuToolkitLibDirs : dossiers de bibliothèques d'un toolkit CUDA ou ROCm
// installé sur la machine, souvent absents du cache du linker. Placés APRÈS les
// bibliothèques livrées avec le moteur : ils ne servent que de complément (la
// variante ROCm s'appuie sur le ROCm du système).
func gpuToolkitLibDirs() []string {
	var dirs []string
	seen := map[string]bool{}
	add := func(d string) {
		if d != "" && !seen[d] && isDir(d) {
			seen[d] = true
			dirs = append(dirs, d)
		}
	}
	add("/usr/local/cuda/lib64")
	add("/usr/local/cuda/targets/x86_64-linux/lib")
	// Installs versionnés, du plus récent au plus ancien (tri sémantique :
	// cuda-12.10 > cuda-12.4, ce qu'un tri de chaînes rate).
	versioned, _ := filepath.Glob("/usr/local/cuda-*/lib64")
	sortByVersionDesc(versioned)
	for _, d := range versioned {
		add(d)
	}
	// Toolkit hors /usr/local (Arch : /opt/cuda, ou CUDA_PATH / CUDA_HOME).
	for _, root := range []string{os.Getenv("CUDA_PATH"), os.Getenv("CUDA_HOME"), "/opt/cuda"} {
		if root != "" {
			add(filepath.Join(root, "lib64"))
		}
	}
	return dirs
}

// autoInstallTool installe un outil de compilation manquant avec le gestionnaire
// de paquets du système (apt/dnf/pacman sous Linux, brew sous macOS). Best-effort :
// renvoie une erreur quand aucun gestionnaire connu n'est disponible ou quand
// l'installation échoue. Utilise sudo sous Linux quand on n'est pas déjà root
// (brew, lui, refuse de tourner en root).
//
// RETABLI le 21/11 : cette fonction existait chez l'amont et a été AVALEE par ma
// fusion 93c24d0 du 14/11, qui a recollé sa derniere ligne a la fin de
// gpuToolkitLibDirs ci-dessus. Le fichier ne compilait plus sur aucun unix depuis
// cette fusion ; Windows ne le voyait pas, le fichier etant exclu de sa compilation.
func autoInstallTool(name string) error {
	managers := []struct {
		bin     string
		install []string // args before the package name
	}{
		{"apt-get", []string{"install", "-y"}},
		{"dnf", []string{"install", "-y"}},
		{"pacman", []string{"-S", "--noconfirm"}},
		{"brew", []string{"install"}},
	}
	for _, m := range managers {
		if _, err := exec.LookPath(m.bin); err != nil {
			continue
		}
		argv := append(append([]string{m.bin}, m.install...), name)
		if m.bin != "brew" && os.Geteuid() != 0 {
			if _, err := exec.LookPath("sudo"); err != nil {
				return fmt.Errorf("%s requiert root (ni root ni sudo disponibles)", m.bin)
			}
			argv = append([]string{"sudo"}, argv...)
		}
		cmd := exec.Command(argv[0], argv[1:]...)
		cmd.Stdout, cmd.Stderr, cmd.Stdin = os.Stdout, os.Stderr, os.Stdin
		return cmd.Run()
	}
	return fmt.Errorf("aucun gestionnaire de paquets connu — installe %s manuellement", name)
}

// refreshToolPath est vide cote unix : les gestionnaires de paquets installent
// dans des dossiers deja au PATH (/usr/bin, /usr/local/bin), contrairement a
// Windows ou il faut relire le registre.
func refreshToolPath() {}

// cudaPathEnv est vide cote unix : aucun environnement CUDA a injecter dans un
// processus fils, les bibliotheques sont trouvees par gpuToolkitLibDirs.
func cudaPathEnv(toolkitDir string) []string { return nil }

// ensureCudaVSIntegration est propre a Windows (l'integration MSBuild de CUDA,
// verifiee aupres de Visual Studio). Aucun equivalent cote unix.
func ensureCudaVSIntegration(toolkitDir string) error { return nil }

// msvcDevEnv est propre a Windows : le generateur Ninja a besoin de
// l'environnement MSVC pose par vcvars. Cote unix la chaine d'outils est deja
// au PATH, il n'y a rien a injecter. La fonction n'existe que pour que le code
// de compilation partage compile.
func msvcDevEnv() ([]string, error) { return nil, nil }

// ensureCompiler makes sure a C/C++ toolchain (cc + c++ + make) is present,
// installing it via the system package manager when missing. build-essential on
// Debian/Ubuntu pulls the lot; elsewhere we fall back to individual packages.
func ensureCompiler() error {
	haveCC := hasTool("cc") || hasTool("gcc") || hasTool("clang")
	haveCXX := hasTool("c++") || hasTool("g++") || hasTool("clang++")
	if haveCC && haveCXX && hasTool("make") {
		return nil
	}
	if parallelBuild() {
		return fmt.Errorf("prototype portable : compilateur C/C++ et make requis ; les fournir séparément ou choisir un moteur précompilé")
	}
	candidates := []string{"build-essential", "gcc", "g++", "make"}
	if _, err := exec.LookPath("apt-get"); err != nil {
		// Non-Debian: build-essential n'existe pas, on vise les paquets directs.
		candidates = []string{"gcc", "gcc-c++", "make"}
	}
	fmt.Println(yellow("[info]") + " compilateur C/C++ absent — installation via le gestionnaire de paquets…")
	for _, pkg := range candidates {
		_ = autoInstallTool(pkg) // best-effort, paquets variables selon la distro
	}
	if (hasTool("cc") || hasTool("gcc")) && (hasTool("c++") || hasTool("g++")) && hasTool("make") {
		fmt.Println(green("✓") + " compilateur C/C++ prêt.")
		return nil
	}
	return fmt.Errorf("compilateur C/C++ introuvable — installe gcc/g++/make (ou build-essential) manuellement")
}

// execServer replaces the current process with llama-server (so systemd
// supervises llama-server directly, as the old start.sh did with `exec`).
// args[0] must be the binary path.
func execServer(bin string, args []string) error {
	return syscall.Exec(bin, args, os.Environ())
}

// newShellCmd builds the command used by the run_shell tool. The second return
// value is a cleanup func the caller must defer (a no-op here; on Windows it
// removes the temporary .bat file — see the Windows twin).
func newShellCmd(ctx context.Context, command string) (*exec.Cmd, func()) {
	return exec.CommandContext(ctx, "/bin/bash", "-c", command), func() {}
}

// ramUsageMB renvoie (utilisée, totale) en Mo pour /api/ram. Linux : /proc/meminfo.
// macOS : sysctl pour le total, vm_stat pour ce qui est réellement libre (pages
// free + inactive + speculative ; le reste — wired, active, compressé — est
// considéré occupé, comme le fait le Moniteur d'activité).
func ramUsageMB() (used, total int) {
	if runtime.GOOS == "darwin" {
		total = int(totalRAMGB() * 1024)
		if total == 0 {
			return 0, 0
		}
		out, err := exec.Command("vm_stat").Output()
		if err != nil {
			return 0, total
		}
		pageSize := 4096
		freePages := 0
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(line, "Mach Virtual Memory Statistics") {
				// « … (page size of 16384 bytes) » — l'Apple Silicon n'est pas en 4 Ko.
				if i := strings.Index(line, "page size of "); i >= 0 {
					if v, err := strconv.Atoi(strings.Fields(line[i+len("page size of "):])[0]); err == nil {
						pageSize = v
					}
				}
				continue
			}
			k, v, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}
			switch strings.TrimSpace(k) {
			case "Pages free", "Pages inactive", "Pages speculative":
				if n, err := strconv.Atoi(strings.Trim(strings.TrimSpace(v), ".")); err == nil {
					freePages += n
				}
			}
		}
		freeMB := freePages * pageSize / (1024 * 1024)
		if freeMB > total {
			freeMB = total
		}
		return total - freeMB, total
	}
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	var totalKB, availKB int
	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		v, _ := strconv.Atoi(f[1]) // kB
		switch f[0] {
		case "MemTotal:":
			totalKB = v
		case "MemAvailable:":
			availKB = v
		}
	}
	if totalKB == 0 {
		return 0, 0
	}
	return (totalKB - availKB) / 1024, totalKB / 1024
}

// --- Supervision de processus détachés (worker de lien, service en mode
// utilisateur). Pendant Unix de sys_platform_windows.go.

// spawnDetached prépare une commande qui survivra à la mort de AJEAN : Setsid la
// place dans une nouvelle session, donc elle n'est pas tuée avec notre groupe.
func spawnDetached(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd
}

// pidAlive : le signal 0 ne tue rien, il teste juste l'existence du process.
func pidAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}

// killTree arrête le process ET ses enfants. Setsid ayant fait de lui un chef de
// groupe, un PID négatif vise le groupe entier ; SIGKILL en dernier recours.
func killTree(pid int) {
	if pid <= 0 {
		return
	}
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
		_ = syscall.Kill(pid, syscall.SIGTERM)
	}
	for i := 0; i < 30 && pidAlive(pid); i++ {
		time.Sleep(100 * time.Millisecond)
	}
	if pidAlive(pid) {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
	}
}
