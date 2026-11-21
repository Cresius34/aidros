package ajean

// sys_marque_os.go — ce que Windows, macOS et le Planificateur de tâches
// affichent de nous HORS de l'interface web : l'infobulle de l'icône de la zone
// de notification, la colonne « Description » du gestionnaire des tâches, le
// libellé du raccourci.
//
// POURQUOI ce fichier existe séparément, et pourquoi les libellés ne restent pas
// écrits dans sys_tray_windows.go : une chaîne d'affichage posée dans une
// fonction bloquante (systray.Run) n'est JAMAIS lue par un test. C'est ainsi
// qu'un champ de 69 caractères est devenu un changelog de 1 043 : la ressource
// Windows cmd/ajean-memory-test/versioninfo.json porte FileDescription, et
// Windows s'en sert comme NOM dans le gestionnaire des tâches. Mesure du 21/11 :
// 949 caractères rendus dans la colonne, sur trois lignes, à la place d'un nom.
// Rien ne le gardait — « git grep FileDescription -- *.go » ne rendait AUCUN
// lecteur. Un champ sans garde grossit, comme le prompt système avant
// TestSystemPromptStaysLean.
//
// LA RÈGLE QUE CES LIBELLÉS POSÈNT, et que le filet vérifie : un libellé d'OS
// porte UN NOM, pas une explication. L'infobulle d'une icône et la colonne
// d'une liste de processus se lisent en un coup d'œil ; tout ce qui ne tient pas
// dans un coup d'œil y devient du bruit, et le bruit empêche de voir LE nom —
// précisément ce que l'oeil cherchait. Le port, le millésime, la description du
// moteur : ils ont leur page (l'onglet Statut, /api/status, le tableau de bord).
//
// Le nom vient de uiBadgeLabel (parallel_profile.go) : une seule source. Le
// renommer une seconde fois ne doit pas demander de retoucher quatre fichiers.
import "fmt"

// libellesTray : les cinq chaînes que l'icône de la zone de notification montre.
// Séparé de runTray pour être TESTABLE — runTray bloque jusqu'à « Quitter » et
// exige une session graphique, donc aucune de ces chaînes n'était vérifiable.
//
// L'infobulle ne porte QUE LE NOM (ordre de l'humain du 21/11, capture à
// l'appui : « laisse que le nom quand on survole l'icône »). Elle ne porte plus
// le port : c'était la seule trace visible de l'instance sur ce poste, et le
// coût est dit honnêtement — deux instances côte à côte se distingueraient par
// leur titre de fenêtre, leur port reste lisible dans l'onglet Statut et dans
// /api/status. Ce n'est pas un gain gratuit, c'est un choix de l'humain, écrit
// ici pour que le prochain qui cherche le port ne le cherche pas sous l'horloge.
func libellesTray() (titre, infobulle, ouvrir, quitter, avisQuitter string) {
	nom := uiBadgeLabel
	if nom == "" {
		// Le build normal ne pose pas de badge, mais il doit quand même nommer
		// son icône : un libellé vide laisse une infobulle vide, et une infobulle
		// vide ne dit pas qu'Aidros tourne.
		nom = "Aidros"
	}
	return nom, nom,
		"Ouvrir " + nom,
		"Quitter " + nom,
		fmt.Sprintf("Arrêter %s et décharger le modèle", nom)
}
