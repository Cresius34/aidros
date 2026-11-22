package aidros

// Le renommage du binaire installé (ajean.exe → aidros.exe) ne doit rien casser
// chez celui qui a installé avant. Deux promesses, deux filets, joués sur de
// vrais fichiers :
//
//   1. une machine installée AVANT garde son « ajean.exe », et il est À JOUR ;
//   2. une machine installée APRÈS ne reçoit pas d'alias — pas 20 Mo payés pour
//      un nom qu'elle n'a jamais connu.
//
// Sans le 1, le raccourci posé depuis des mois relance un binaire périmé à chaque
// clic (le piège déjà écrit plus bas dans sys_install_windows.go). Sans le 2, le
// renommage double le poids de chaque installation neuve.
//
// Le 1 est celui qui a un passé : c'est le cas vécu. Le 2 est celui qui protège
// l'avenir. Les deux se contredisent, et c'est la ligne « mettre à jour s'il
// existe, ne pas créer » qui les réconcilie — elle est jouée ici des deux côtés.

import (
	"os"
	"path/filepath"
	"testing"
)

// TestAliasHeriteMisAJour : la machine d'avant. Son « ajean.exe » est là, figé
// sur une version périmée. installSelf doit le remettre à niveau, parce que le
// raccourci et le PATH du poste visent ce nom.
func TestAliasHeriteMisAJour(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "bin")
	alias := filepath.Join(bin, "ajean.exe")

	// L'installation d'avant : l'alias existe, et il est PÉTIMÉ (un fichier de
	// quelques octets, alors que le binaire qui tourne en pèse des dizaines de Mo).
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(alias, []byte("ancienne version"), 0o755); err != nil {
		t.Fatal(err)
	}

	dst, err := installSelf(bin)
	if err != nil {
		t.Fatalf("installSelf : %v", err)
	}

	// Le canonique est écrit.
	canonique, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("binaire canonique : %v", err)
	}
	// Et l'alias porte DÉSORMAIS LA MÊME chose : le raccourci qui le vise ne
	// relancera plus une version périmée.
	relus, err := os.ReadFile(alias)
	if err != nil {
		t.Fatalf("alias après entretien : %v", err)
	}
	if len(relus) != len(canonique) {
		t.Errorf("alias = %d octets, le canonique en fait %d : l'alias est resté périmé", len(relus), len(canonique))
	}
	if string(relus) == "ancienne version" {
		t.Error("l'alias porte encore « ancienne version » : le raccourci relancerait un vieux binaire")
	}
}

// TestAliasNonCree : la machine d'après. Rien ne s'appelait ajean ici, il ne faut
// donc pas en créer un.
func TestAliasNonCree(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "bin")

	dst, err := installSelf(bin)
	if err != nil {
		t.Fatalf("installSelf : %v", err)
	}
	if _, err := os.Stat(dst); err != nil {
		t.Fatalf("binaire canonique non écrit : %v", err)
	}

	alias := filepath.Join(bin, "ajean.exe")
	if _, err := os.Stat(alias); err == nil {
		t.Errorf("%s a été CRÉÉ alors que cette machine ne l'a jamais eu : chaque installation neuve paierait deux copies du binaire", alias)
	}
}

// TestCanoniqueNommeLeProduit : le renommage est bien LÀ. Un renommage qui
// n'écrit pas le nom attendu n'est pas un renommage — et le nom du fichier est
// ce que tapent le PATH, le raccourci et le lanceur.
func TestCanoniqueNommeLeProduit(t *testing.T) {
	bin := filepath.Join(t.TempDir(), "bin")
	dst, err := installSelf(bin)
	if err != nil {
		t.Fatalf("installSelf : %v", err)
	}
	if nom := filepath.Base(dst); nom != "aidros.exe" {
		t.Errorf("binaire installé nommé %q, attendu « aidros.exe »", nom)
	}
}
