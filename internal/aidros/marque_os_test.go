package aidros

// TestLibellesTrayPortentUnNom — ce que Windows affiche de nous HORS de
// l'interface web doit rester un NOM.
//
// POURQUOI ce filet existe, en deux mots : parce que le champ ne coûtait rien.
// « git grep FileDescription -- *.go » rendait AUCUN lecteur dans le dépôt, donc
// rien ne s'opposait à ce qu'il grossisse. Il a grossi : 69 caractères chez
// l'amont, 1 043 chez nous (mesure du 21/11 sur cmd/ajean-memory-test/versioninfo.json),
// 949 dans les deux binaires en service (PID 58280 et 87164).
//
// Ce que ça coûtait, et pourquoi ce n'est pas de l'esthétique : Windows ne montre
// PAS le nom de l'exécutable dans la colonne « Description » du gestionnaire des
// tâches — il montre le FileDescription. La capture de l'humain du 21/11 le
// rend : « Firefox (29) », « Python », « Secure System » sont des descriptions.
// Le nôtre y tenait TROIS LIGNES, à la place du nom que l'œil cherchait. Même
// cause pour l'infobulle de l'icône : elle disait le port, pas le nom.
//
// Le garde porte donc sur la TAILLE et sur la FORME, pas sur le sens : « que le
// nom » est une règle de longueur avant d'être une règle de rédaction, et une
// longueur est vérifiable. 40 caractères : assez pour un nom et son millésime,
// trop court pour un paragraphe.
//
// ⚠ Contre-épreuve jouée dans les deux sens, comme toujours : bornes trop basses
// → rouge ; libellé rallongé → rouge ; remis → vert. Un garde qu'on n'a jamais
// vu rouge n'est pas un garde, c'est une ligne qui passe.
import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// longueurLibelleOs : la borne au-delà de laquelle un champ d'affichage Windows
// cesse d'être un nom. 40 = le nom + le millésime + de la marge. La borne est
// haussable, elle n'est PAS baissable sans raison : c'est vers le haut que ces
// champs grossissent, ligne par ligne, chacun paraissant anodin — exactement le
// défaut de TestSystemPromptStaysLean, dans un autre champ.
const longueurLibelleOs = 40

// UNE SEULE BORNE depuis le 21/11 au soir. Il y en avait DEUX, et la seconde est
// morte de sa propre justification — à raconter, parce que sa raison était bonne :
//
// Le 21/11, ce garde est tombé ROUGE à son premier essai sur cmd/ajean, dont le
// FileDescription faisait 73 caractères (« AJEAN — gestionnaire mono-binaire de
// serveurs llama.cpp auto-hébergés »). Ce rouge m'instruisait : le texte était
// celui de L'AMONT. J'ai donc posé une borne séparée à 120, la raison étant une
// question de PROPRIÉTÉ et non de taille — imposer ma plume sur la phrase d'un
// autre ne rend l'affichage meilleur pour personne, et nous éloigne de la
// doctrine « fidèle à l'officiel, nos modifications AUTOUR ». 120 : assez long
// pour sa phrase, cent fois trop court pour un changelog.
//
// Le 21/11 au soir, l'humain a demandé explicitement le nom sur cmd/ajean
// (« remplacé le nom ajean du cmd/ajean »). La retenue n'était mienne que faute
// d'ordre ; un ordre de l'humain la lève. Les deux points d'entrée portent donc
// « Aidros » (6 caractères mesurés) et la borne unique est longueurLibelleOs.
//
// ⚠ La borne n'est PAS baissable sans raison, et elle ne protège pas un nom :
// ces champs grossissent vers le HAUT, ligne par ligne, chacun paraissant
// anodin. Le texte d'amont retiré est recopié dans le message du commit qui le
// retire — c'est sa place, git log est consultable.

func TestLibellesTrayPortentUnNom(t *testing.T) {
	titre, infobulle, ouvrir, quitter, avis := libellesTray()

	for nom, s := range map[string]string{
		"titre de l'icône":    titre,
		"infobulle":           infobulle,
		"élément « Ouvrir »":  ouvrir,
		"élément « Quitter »": quitter,
		"avis de « Quitter »": avis,
	} {
		if s == "" {
			t.Errorf("%s : vide. Une infobulle vide ne dit pas qu'Aidros tourne.", nom)
			continue
		}
		if len(s) > longueurLibelleOs {
			t.Errorf("%s = %d caractères (%q), borne %d — un libellé d'OS porte un NOM, pas une explication. Le détail a sa page : onglet Statut, /api/status, le tableau de bord.",
				nom, len(s), s, longueurLibelleOs)
		}
	}

	// « Laisse que le nom » (ordre du 21/11, capture à l'appui) : l'infobulle ne
	// doit plus porter le port. C'est la demande, donc c'est le garde. Le port
	// reste lisible dans l'onglet Statut et dans /api/status.
	if strings.ContainsAny(infobulle, "0123456789") {
		t.Errorf("infobulle = %q : elle porte un chiffre, donc probablement un port. L'ordre du 21/11 est le nom seul.", infobulle)
	}

	// Le nom vient d'une seule source. Si uiBadgeLabel change, l'infobulle suit :
	// un libellé recopié à la main divergerait sans rien dire.
	if titre != uiBadgeLabel {
		t.Errorf("titre de l'icône = %q, attendu %q (uiBadgeLabel) : deux sources pour un seul nom", titre, uiBadgeLabel)
	}
}

// TestLibelleRaccourciPorteUnNom : le texte que Windows affiche au survol du
// raccourci posé dans le menu Démarrer et sur le Bureau.
//
// Il vit à l'intérieur d'une chaîne brute Go (le script PowerShell embarqué par
// sys_firstrun_windows.go), donc AUCUN test ne pouvait l'atteindre jusqu'ici :
// il disait « AJEAN, votre IA locale » depuis l'extérieur de toute vérification.
// Le lire depuis le fichier source est fragile au formatage — c'est le prix
// d'un libellé enfoui dans un script ; le garde dit « la règle a été touchée »,
// il ne dit pas « l'affichage est correct ».
//
// La chaîne est cherchée des DEUX côtés du bloc, et le filet exige de la trouver
// DEUX fois : une fois pour le raccourci qui existe déjà (le poste de l'humain,
// posé depuis des mois), une fois pour la création. Un seul des deux serait un
// correctif qui ne s'applique jamais à la machine qu'il visait.
func TestLibelleRaccourciPorteUnNom(t *testing.T) {
	b, err := os.ReadFile("../../internal/aidros/sys_firstrun_windows.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(b)

	const pose = "$s.Description='Aidros'"
	if n := strings.Count(src, pose); n != 2 {
		t.Errorf("libellé du raccourci : %q trouvé %d fois, attendu 2 (le raccourci existant ET la création). Un seul = le correctif ne touche jamais la machine où le raccourci est posé depuis des mois.", pose, n)
	}
	for _, reste := range []string{"$s.Description='AJEAN"} {
		if strings.Contains(src, reste) {
			t.Errorf("l'ancien libellé est encore posé : %q", reste)
		}
	}
}

// TestRessourceWindowsPorteUnNom : FileDescription et ProductName des deux
// points d'entrée. Windows lit FileDescription comme NOM dans le gestionnaire
// des tâches ; ProductName comme « nom du produit » dans l'Explorateur.
//
// Les deux fichiers portent DÉJÀ nos modifications (mesuré contre
// upstream/ajean-v0.18.17 : +7/−7 et +38/−0), donc les garder n'ajoute aucun
// conflit de fusion.
func TestRessourceWindowsPorteUnNom(t *testing.T) {
	// Une seule borne depuis le 22/11 : les deux points d'entrée portent le nom
	// (voir longueurLibelleOs, et ci-dessus l'histoire de la seconde borne).
	for _, c := range []struct {
		chemin string
		borne  int
	}{
		{"../../cmd/ajean/versioninfo.json", longueurLibelleOs},
		{"../../cmd/ajean-memory-test/versioninfo.json", longueurLibelleOs},
	} {
		b, err := os.ReadFile(c.chemin)
		if err != nil {
			t.Fatal(err)
		}
		var vi struct {
			StringFileInfo struct{ FileDescription, ProductName string }
		}
		if err := json.Unmarshal(b, &vi); err != nil {
			t.Fatal(err)
		}
		for champ, s := range map[string]string{
			"FileDescription": vi.StringFileInfo.FileDescription,
			"ProductName":     vi.StringFileInfo.ProductName,
		} {
			if s == "" {
				t.Errorf("%s : %s vide", c.chemin, champ)
				continue
			}
			if len(s) > c.borne {
				t.Errorf("%s : %s = %d caractères, borne %d. Ce champ est le NOM que Windows affiche (colonne Description du gestionnaire des tâches, propriétés de l'exe) : un changelog y devient trois lignes de bruit. Le changelog vit dans les messages de commit et dans RELEASE_NOTES.md.\nPremiers 60 : %q",
					c.chemin, champ, len(s), c.borne, s[:min(60, len(s))])
			}
		}
	}
}
