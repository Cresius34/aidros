package aidros

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const uiUnitName = "ajean-ui"

func uiServiceName() string { return uiUnitName }

// cmdUI pilote le service de l'INTERFACE (l'unité systemd « ajean-ui », ou le
// worker détaché ailleurs). Il logeait dans relay_link.go parce que le tunnel y
// habitait ; le relais retiré, ce service survit — c'est lui qui tient
// l'interface de l'application.
//
// Le tunnel, lui, n'est plus un service à part : il était ouvert par
// « aidros web » dès qu'un jeton de liaison était enregistré. C'est ce qui
// garantissait une conversation unique, identique en local et à distance.
func cmdUI(args []string) error {
	action := "status"
	if len(args) > 0 && args[0] != "" {
		action = args[0]
	}
	switch action {
	case "start", "stop", "restart":
		return uiServiceCtl(action)
	case "status":
		if uiServiceActive() {
			fmt.Printf("%s service %s : actif\n", green("[ok]"), uiServiceName())
		} else {
			fmt.Printf("%s service %s : arrêté\n", yellow("[info]"), uiServiceName())
		}
		return nil
	default:
		return fmt.Errorf("usage: aidros ui [start|stop|restart|status]")
	}
}

// killForeignUIWorker tue un worker de lien détaché encore en vie (laissé par
// une version antérieure ou une autre copie de l'app). Indispensable AVANT que
// l'app ne prenne la main : sinon deux process servent la même conversation et
// les fils divergent entre l'UI locale et app.ajean.link.
func killForeignUIWorker() {
	if pid := uiUserPID(); pid > 0 {
		killTree(pid)
		_ = os.Remove(uiPIDPath())
	}
}

// uiPIDPath / uiLogPath : suivi du worker hors systemd (macOS, Windows), où
// Aidros est une app de bureau lancée sans droits root.
func uiPIDPath() string { return filepath.Join(AjeanHome(), uiUnitName+".pid") }
func uiLogPath() string { return filepath.Join(AjeanHome(), uiUnitName+".log") }

// uiServiceCtl pilote le service de l'interface (start/stop/restart). Sous
// Linux c'est l'unité systemd ajean-ui (avec sudo non interactif si on n'est
// pas root) ; ailleurs — macOS et Windows, où il n'y a ni systemd ni droits
// root — on lance « aidros web » en processus détaché suivi par un fichier PID,
// comme le fait déjà le service principal. Sans ça, le service restait
// définitivement « arrêté » sur un Mac ou un PC.
//
// La branche « app de bureau » qui logeait ici pilotait le TUNNEL du relais :
// elle ouvrait la boucle de lien dans le processus même, pour une conversation
// unique. Le relais retiré, il ne reste que le service, tel quel.
func uiServiceCtl(action string) error {
	if runtime.GOOS != "linux" {
		return uiUserSvcCtl(action)
	}
	bin, pre := "systemctl", []string{}
	if os.Geteuid() != 0 {
		bin, pre = "sudo", []string{"-n", "systemctl"}
	}
	cmd := exec.Command(bin, append(pre, action, uiServiceName())...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("systemctl %s %s: %w", action, uiServiceName(), err)
	}
	switch action {
	case "start", "restart":
		fmt.Printf("%s service %s %s\n", green("[ok]"), uiServiceName(), action+"é")
	case "stop":
		fmt.Printf("%s service %s arrêté\n", green("[ok]"), uiServiceName())
	}
	return nil
}

// uiServiceActive indique si le service de l'interface tourne (unité systemd
// sous Linux, processus suivi par fichier PID ailleurs).
func uiServiceActive() bool {
	if runtime.GOOS != "linux" {
		return uiUserPID() > 0
	}
	out, _ := exec.Command("systemctl", "is-active", uiServiceName()).Output()
	return strings.TrimSpace(string(out)) == "active"
}

// uiUserPID renvoie le PID du worker de lien s'il tourne vraiment, 0 sinon
// (fichier absent, illisible, ou process mort → on nettoie le fichier obsolète).
func uiUserPID() int {
	b, err := os.ReadFile(uiPIDPath())
	if err != nil {
		return 0
	}
	if i := strings.IndexByte(string(b), '\n'); i >= 0 {
		b = b[:i]
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	if pid <= 0 || !pidAlive(pid) {
		return 0
	}
	return pid
}

// uiUserSvcCtl : équivalent de systemctl pour les OS sans systemd.
func uiUserSvcCtl(action string) error {
	switch action {
	case "stop", "restart":
		if pid := uiUserPID(); pid > 0 {
			killTree(pid)
			for i := 0; i < 30 && pidAlive(pid); i++ {
				time.Sleep(100 * time.Millisecond)
			}
		}
		_ = os.Remove(uiPIDPath())
		if action == "stop" {
			fmt.Printf("%s service %s arrêté\n", green("[ok]"), uiServiceName())
			return nil
		}
	case "start":
		if uiUserPID() > 0 {
			fmt.Printf("%s service %s déjà en cours\n", yellow("[info]"), uiServiceName())
			return nil
		}
	default:
		return fmt.Errorf("action inconnue: %s", action)
	}

	self, err := os.Executable()
	if err != nil {
		return err
	}
	if p, err := filepath.EvalSymlinks(self); err == nil {
		self = p
	}
	if err := os.MkdirAll(AjeanHome(), 0o755); err != nil {
		return err
	}
	logf, err := os.OpenFile(uiLogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("ouverture du log %s: %w", uiLogPath(), err)
	}
	defer logf.Close()

	cmd := spawnDetached(self, "web")
	cmd.Stdout, cmd.Stderr = logf, logf
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("démarrage de « aidros web »: %w", err)
	}
	pid := cmd.Process.Pid
	// PID + binaire d'origine : la 2e ligne dit QUELLE copie de l'app a lancé ce
	// worker — précieux pour diagnostiquer un process rescapé d'une autre version.
	if err := os.WriteFile(uiPIDPath(), []byte(strconv.Itoa(pid)+"\n"+self+"\n"), 0o644); err != nil {
		return fmt.Errorf("écriture du PID: %w", err)
	}
	_ = cmd.Process.Release()

	// Le worker peut mourir aussitôt (token refusé, relais injoignable) : on laisse
	// passer un instant avant de déclarer la victoire, sinon l'UI afficherait
	// « en ligne » sur un process déjà mort.
	time.Sleep(1500 * time.Millisecond)
	if !pidAlive(pid) {
		_ = os.Remove(uiPIDPath())
		return fmt.Errorf("le lien s'est arrêté aussitôt — voir %s", uiLogPath())
	}
	fmt.Printf("%s service %s démarré (PID %d)\n", green("[ok]"), uiServiceName(), pid)
	return nil
}

// appWebMux : le mux que l”app de bureau sert elle-mÃªme, et qui porte la
// conversation â€” donc un seul processus propriÃ©taire. Le relais le
// rÃ©utilisait pour ouvrir le tunnel sur la MÃŠME conversation ; le relais est
// retirÃ©, le mux reste celui de l”interface locale (cli_app.go).
var appWebMux *http.ServeMux
