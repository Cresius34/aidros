package aidros

// Le NOM DU FICHIER .lnk est le texte que Windows affiche sous l'icône, sur le
// Bureau et dans le menu Démarrer (l'explorateur masque l'extension). Ce fichier
// est le seul endroit où cette promesse est tenue.
//
// Elle n'a rien d'évident, et trois façons de la casser sont venues à l'esprit
// avant de l'écrire :
//
//   · écrire « Aidros.lnk » en dur dans le script PowerShell : le nom aurait deux
//     sources, et la troisième bascule (il y en a déjà eu deux — « jean.exe » puis
//     « ajean.exe ») en laisserait une de côté ;
//   · renommer sans ménagement : un poste installé d'avant se réveille avec
//     « AJEAN.lnk » ET « Aidros.lnk » côte à côte, deux icônes pour une
//     application ;
//   · ne pas toucher au nom d'avant dans removeShortcuts : un orphelin reste sur
//     le Bureau du poste qui a installé avant, et personne ne le retire plus.
//
// Le script PowerShell n'est joué par AUCUN test d'exécution : il tourne sur la
// machine de l'humain. Il est donc lu comme du texte, à la façon de
// TestLibelleRaccourciPorteUnNom, qui fait de même pour le libellé au survol.

import (
	"os"
	"strings"
	"testing"
)

const cheminFirstrun = "../../internal/aidros/sys_firstrun_windows.go"

func TestNomRaccourciSuitLeProduit(t *testing.T) {
	// 1. Le nom vient de la source unique. C'est ce qui empile trois surfaces —
	// le .lnk, l'infobulle système (sys_marque_os.go:42), la boîte de dialogue —
	// sur une seule ligne à retoucher si le nom change encore.
	if voulu := uiBadgeLabel + ".lnk"; nomRaccourci() != voulu {
		t.Errorf("nomRaccourci() = %q, attendu %q : le nom du raccourci a une autre source que uiBadgeLabel", nomRaccourci(), voulu)
	}

	b, err := os.ReadFile(cheminFirstrun)
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)

	// 2. Le script ne pose plus le nom d'avant en CRÉATION. « $lnkVieux= » le
	// nomme pour le RECONNAÎTRE (ménagement) ; l'interdire partout interdirait
	// aussi le ménage. Ce qui est visé : un Join-Path $d 'AJEAN.lnk' qui servirait
	// à créer, et ferait reparaître l'ancien nom sous la nouvelle icône.
	for _, creation := range []string{"Join-Path $d 'AJEAN.lnk'"} {
		if strings.Contains(src, creation) {
			t.Errorf("le script pose encore un raccourci sous le nom d'avant : %q", creation)
		}
	}

	// 3. Le ménage ne doit pas doubler. Sans renommage explicite, le poste installé
	// d'avant garde ses DEUX icônes.
	for _, menage := range []string{"Move-Item $a $lnk", "Remove-Item $a"} {
		if !strings.Contains(src, menage) {
			t.Errorf("le renommage du raccourci n'est plus ménagé (%q absent) : un poste installé d'avant afficherait deux icônes", menage)
		}
	}
}

func TestSuppressionBalaieLesDeuxNoms(t *testing.T) {
	b, err := os.ReadFile(cheminFirstrun)
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)

	// removeShortcuts doit connaître les deux noms : un poste d'avant porte
	// « AJEAN.lnk », un poste d'après porte « Aidros.lnk ». N'en chercher qu'un
	// laisse un orphelin sur le Bureau de l'un ou de l'autre.
	i := strings.Index(src, "func removeShortcuts")
	if i < 0 {
		t.Fatal("removeShortcuts introuvable")
	}
	corps := src[i:]
	if j := strings.Index(corps, "\nfunc "); j > 0 {
		corps = corps[:j]
	}

	for _, nom := range []string{"nomRaccourci()", `"AJEAN.lnk"`} {
		if !strings.Contains(corps, nom) {
			t.Errorf("removeShortcuts ne balaie pas %s : un orphelin resterait sur le Bureau", nom)
		}
	}
}

// TestBoiteDialogueSuitLeProduit : la fenêtre qui s'ouvre après l'installation
// nomme le produit. Elle était la plus fausse du lot : « AJEAN est installé »
// s'affichait sous une application qui s'appelle Aidros, et « Raccourci « AJEAN »
// ajouté » nommait une icône qui porte un autre nom depuis ce même commit.
func TestBoiteDialogueSuitLeProduit(t *testing.T) {
	b, err := os.ReadFile(cheminFirstrun)
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)

	// Les chaînes données à messageBox, pas les commentaires : c'est là que le nom
	// devient visible par l'humain.
	for _, chaine := range []string{
		`messageBox(uiBadgeLabel+" est installé`,
		`"Mise à jour d'"+uiBadgeLabel`,
		`"Raccourci « " + uiBadgeLabel + " »`,
		`"Installation impossible\n\n"+err.Error()+"\n\n"+uiBadgeLabel`,
	} {
		if !strings.Contains(src, chaine) {
			t.Errorf("la boîte de dialogue ne suit plus uiBadgeLabel : %q absent", chaine)
		}
	}

	// L'ancien nom ne doit plus rester dans une CHAÎNE VISIBLE. Les commentaires
	// sont permis — et nécessaires : c'est là que s'écrit pourquoi le nom a changé,
	// et deux lignes de ce fichier en témoignent (« Écrire « AJEAN » ici l'aurait
	// laissée seule à contredire… », « Raccourci « AJEAN » ajouté » sous-entendrait… »).
	// D'où la précaution : on ne sonde QUE ce qui est passé à messageBox. Sonder le
	// fichier entier rendait le garde ROUGE sur mes propres commentaires — un rouge
	// qui ne reprochait rien, et qui m'aurait fait effacer l'explication pour faire
	// taire le filet. C'est l'inverse du travail.
	appels := messageBoxAppels(src)
	for _, reste := range []string{`"AJEAN"`, `"AJEAN est`, `"Mise à jour d'AJEAN"`, "AJEAN ", "AJEAN va", "AJEAN "} {
		if strings.Contains(appels, reste) {
			t.Errorf("une chaîne visible nomme encore l'ancien nom : %q", reste)
		}
	}
}

// messageBoxAppels extrait de la source ce qui est DONNÉ à messageBox, à
// l'exclusion des commentaires. Le nom du produit ne devient visible que là ;
// le reste du fichier a le droit de raconter son histoire.
func messageBoxAppels(src string) string {
	var seulementLesChaines []string
	for _, ligne := range strings.Split(src, "\n") {
		t := strings.TrimSpace(ligne)
		if strings.HasPrefix(t, "//") {
			continue
		}
		// Un commentaire en fin de ligne ne doit pas être pris pour la chaîne.
		if i := strings.Index(t, " // "); i >= 0 {
			t = t[:i]
		}
		if strings.Contains(t, "messageBox") || strings.Contains(t, "uiBadgeLabel") ||
			strings.Contains(t, `"`) {
			seulementLesChaines = append(seulementLesChaines, t)
		}
	}
	return strings.Join(seulementLesChaines, "\n")
}
