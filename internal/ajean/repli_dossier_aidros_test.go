//go:build windows

package ajean

// repli_dossier_aidros_test.go — le dossier de repli s'appelle Aidros, et
// l'ancien reste admis. Garde-fou demandé par l'ordre de l'humain du 21/11 :
// « il cherche le mauvais dossier du coup, fais-lui chercher un dossier
// Aidros ».
//
// Ce que ce test protège n'est pas le nom. C'est l'ordre. Une machine installée
// avant le renommage a sa mémoire, ses clés et sa base sous
// <ProgramData>\ajean : si le repli sautait sur un Aidros inexistant,
// l'application s'ouvrirait SANS MÉMOIRE et sans le dire — un dossier que la
// machine vient de créer ne s'appelle pas « installation vide » tout seul. Le
// défaut de la série Gyro (17/11) était exactement cette forme : un refus
// silencieux, rendu par un chemin que personne ne regardait.
//
// Le test ne démare rien : defaultAjeanHome() ne fait que stat(). homeRoot()
// relit %ProgramData% à CHAQUE APPEL (sys_platform_windows.go:49), donc
// t.Setenv suffit à feindre la racine machine — c'est ce qui rend les quatre
// cas jouables sans toucher au vrai C:\ProgramData.

import (
	"os"
	"path/filepath"
	"testing"
)

// racineFeinte pose un (ou des) dossier(s) de repli sous une racine éphémère et
// renvoie la valeur à donner à %ProgramData%.
func racineFeinte(t *testing.T, noms ...string) string {
	t.Helper()
	racine := t.TempDir()
	for _, n := range noms {
		if err := os.MkdirAll(filepath.Join(racine, n), 0o755); err != nil {
			t.Fatalf("créer %s : %v", n, err)
		}
	}
	t.Setenv("ProgramData", racine)
	return racine
}

// TestRepliChercheAidros : sur une machine neuve, le repli est Aidros.
func TestRepliChercheAidros(t *testing.T) {
	racine := racineFeinte(t) // aucun dossier de repli
	got := defaultAjeanHome()
	voulu := filepath.Join(racine, "Aidros")
	if got != voulu {
		t.Fatalf("repli d'une machine neuve : obtenu %q, voulu %q", got, voulu)
	}
	// Le nom d'avant ne doit plus être celui d'une machine neuve : sinon le
	// renommage n'a rien fait.
	if got == filepath.Join(racine, "ajean") {
		t.Fatal("le repli cherche toujours « ajean » sur une machine neuve")
	}
}

// TestRepliAdmetAjean : la machine d'avant garde sa mémoire. C'est LE cas qui
// fait la valeur du garde-fou — sans lui, le renommage est une perte de données
// silencieuse chez l'utilisateur installé.
func TestRepliAdmetAjean(t *testing.T) {
	racine := racineFeinte(t, "ajean")
	got := defaultAjeanHome()
	if voulu := filepath.Join(racine, "ajean"); got != voulu {
		t.Fatalf("machine installée avant le renommage : obtenu %q, voulu %q — sa mémoire serait abandonnée", got, voulu)
	}
}

// TestRepliAidrosGagneSurLesDeux : le renommage de l'ancien en Aidros est LE
// geste de bascule, et il doit suffire seul. L'ancien peut rester en place le
// temps du ménage : il ne doit pas avoir gagné par ancienneté.
func TestRepliAidrosGagneSurLesDeux(t *testing.T) {
	racine := racineFeinte(t, "ajean", "Aidros")
	got := defaultAjeanHome()
	if voulu := filepath.Join(racine, "Aidros"); got != voulu {
		t.Fatalf("les deux existent : obtenu %q, voulu %q", got, voulu)
	}
}

// TestRepliNeCreePasUnFauxVide : le repli ne doit jamais désigner un Aidros
// vide PENDANT que l'ancien, lui, est plein. C'est la forme du rouge à ne plus
// revoir : l'app s'ouvre, rien ne se plaint, la mémoire a disparu.
func TestRepliNeCreePasUnFauxVide(t *testing.T) {
	racine := racineFeinte(t, "ajean")
	// On garnit l'ancien comme une vraie installation : la base, le coffre, les
	// journaux. Un simple dossier vide ne prouverait rien.
	for _, f := range []string{"ajean.db", ".keyvault", "ajean-engine.log"} {
		if err := os.WriteFile(filepath.Join(racine, "ajean", f), []byte("x"), 0o600); err != nil {
			t.Fatalf("poser %s : %v", f, err)
		}
	}
	got := defaultAjeanHome()
	if filepath.Base(got) != "ajean" {
		t.Fatalf("l'installation garnie est abandonnée au profit de %q : la mémoire de l'humain serait perdue en silence", filepath.Base(got))
	}
	// Et le chemin désigné contient bien les données.
	if !isFile(filepath.Join(got, "ajean.db")) {
		t.Fatalf("le repli %q ne mène pas à la base", got)
	}
}

// TestRepliSousAJEANHome : la variable d'environnement gagne sur les deux noms.
// Elle est ce que pose le lanceur de l'humain (lanceur RX6900XT, service de
// l'opérateur) ; le renommage ne doit pas la court-circuiter.
func TestRepliSousAJEANHome(t *testing.T) {
	racine := racineFeinte(t, "ajean", "Aidros")
	cible := filepath.Join(racine, "ailleurs")
	t.Setenv("AJEAN_HOME", cible)
	if got := AjeanHome(); got != cible {
		t.Fatalf("$AJEAN_HOME devrait gagner : obtenu %q, voulu %q", got, cible)
	}
}

// TestRepliContreEpreuve : le filet doit rougir si l'admission de l'ancien est
// retirée. Jouée sur le comportement, pas sur l'intention : on laisse les deux
// dossiers absents et on vérifie que le cas « ajean garni » est bien SÉLECTIONNÉ
// par le test ci-dessus, pas par hasard.
func TestRepliContreEpreuve(t *testing.T) {
	racine := racineFeinte(t)
	// Un dossier « ajean » qui n'existe PAS ne doit surtout pas être admis :
	// sinon le test précédent réussirait pour une raison sans rapport.
	if got := defaultAjeanHome(); got != filepath.Join(racine, "Aidros") {
		t.Fatalf("machine sans aucun repli : obtenu %q", got)
	}
	// Le nom d'avant, présent sous une casse différente (Windows n'est pas
	// sensible à la casse), doit être admis lui aussi : c'est ce qui évite
	// qu'un « Ajean » existant passe pour un dossier sans rapport.
	if err := os.MkdirAll(filepath.Join(racine, "Ajean"), 0o755); err != nil {
		t.Fatalf("créer Ajean : %v", err)
	}
	if got := defaultAjeanHome(); got == filepath.Join(racine, "Aidros") {
		t.Fatal("un dossier « Ajean » existant n'est pas admis : le repli repartirait sur du vide")
	}
}
