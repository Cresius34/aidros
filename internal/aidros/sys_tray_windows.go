//go:build windows

package aidros

// sys_tray_windows.go — icône de AJEAN dans la zone de notification Windows (system
// tray). Quand on lance l'app (double-clic ou `aidros app`), l'UI web s'ouvre dans
// le navigateur ET une petite icône apparaît près de l'horloge : elle montre que
// AJEAN tourne et offre un menu « Ouvrir Aidros » / « Quitter ».
//
// getlantern/systray est pur Go sur Windows (Win32 via x/sys, aucun CGO) — le
// binaire reste statique. L'équivalent macOS vit dans sys_tray_darwin.go (Cocoa,
// donc CGO) ; les builds Linux ne touchent jamais systray.

import (
	"os"

	"github.com/getlantern/systray"
)

// runTray ouvre l'UI dans le navigateur puis fait tourner l'icône de la zone de
// notification. Bloque jusqu'à ce que l'utilisateur choisisse « Quitter ».
func runTray(url string) {
	systray.Run(func() {
		// Les cinq libellés viennent d'une fonction, pas de littéraux ici :
		// runTray bloque jusqu'à « Quitter » et exige une session graphique,
		// donc rien de ce qui s'affiche sous l'horloge n'était vérifiable. Les
		// libellés vivent dans sys_marque_os.go, gardés par
		// TestLibellesTrayPortentUnNom.
		//
		// L'infobulle ne dit PLUS le port. Elle le disait (« … — port 18090 »)
		// et l'humain a demandé le nom seul, capture à l'appui. Coût assumé et
		// nommé : le port n'est plus lisible sous l'horloge ; il reste dans
		// l'onglet Statut et dans /api/status.
		title, tip, ouvrir, quitter, avisQuitter := libellesTray()
		if parallelBuild() {
			// Le build portable n'arrête pas le moteur en quittant (il ne
			// l'arrête pas : voir la fonction de sortie). Le dire ici vaut
			// mieux qu'un libellé qui promet « décharger le modèle ».
			avisQuitter = "Arrêter cette copie et son encodeur"
		}
		systray.SetIcon(trayIcon())
		systray.SetTitle(title)
		systray.SetTooltip(tip)
		mOpen := systray.AddMenuItem(ouvrir, "Ouvrir l'interface")
		systray.AddSeparator()
		mQuit := systray.AddMenuItem(quitter, avisQuitter)

		go func() {
			for {
				select {
				case <-mOpen.ClickedCh:
					// Fenêtre déjà ouverte : on la ramène, on n'en ouvre pas une seconde.
					if !focusAppWindow() {
						_ = openAppWindow(url)
					}
				case <-mQuit.ClickedCh:
					systray.Quit()
					return
				}
			}
		}()
	}, func() {
		// « Quitter » doit tout arrêter, moteur compris.
		//
		// Le moteur tourne dans un process DÉTACHÉ (voir svcStart) : il survit
		// délibérément à la fermeture de l'app pour que le modèle reste chargé
		// entre deux ouvertures de l'interface. Mais quand on quitte pour de
		// bon, il n'a plus rien pour le piloter et garde des dizaines de Go de
		// RAM, sans la moindre fenêtre pour l'expliquer. svcStop tue l'arbre
		// (taskkill /T), donc llama-server avec lui.
		//
		// Windows seulement : ici l'app EST le propriétaire du moteur. Sous
		// Linux et macOS c'est systemd ou launchd, et fermer une interface n'a
		// pas à arrêter un service système.
		if parallelBuild() {
			stopEmbeddingServer()
			_ = parallelServiceAction("stop")
		} else {
			_ = svcStop(false)
		}
		os.Exit(0)
	})
}

// trayIcon : l'icône de marque en .ico, deux tailles pour rester nette selon
// l'échelle d'affichage. Le rendu vit dans sys_brand_icon.go, partagé avec
// l'icône du .exe.
func trayIcon() []byte { return BrandICO(16, 20, 24, trayIconSize, 40, 48) }
