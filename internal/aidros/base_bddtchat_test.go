package aidros

// base_bddtchat_test.go — la base s'appelle bddtchat.db, et la mémoire de
// l'ancien nom n'est jamais abandonnée en silence.
//
// Ordre de l'humain du 21/11 : « ajean.db devra devenir bddtchat.db, ne renomme
// pas le fichier, on le fera lors de la bascule, mais les traces de "Aidros"
// doivent être modifiées ».
//
// Ce que ce test protège n'est pas le nom. C'est ce que bbolt fait d'un chemin
// qui n'existe pas : bolt.Open CRÉE une base vide, sans un bruit. Point sur
// « bddtchat.db » un jour où le fichier s'appelle encore « ajean.db », et
// l'application redémarre sur une base vide — 536 Mo de mémoire, de
// conversations, de clés et de réglages abandonnés, avec zéro message. C'est la
// troisième fois de la journée que je règle ce genre de défaut (le dossier de
// repli, le FileDescription, celui-ci) : à chaque fois le symptôme était
// « rien ne se plaint », jamais « une erreur ».
//
// Le test ne touche PAS la vraie base : chaque cas pose son AJEAN_HOME sous un
// dossier éphémère, et dbPath() ne fait que stat() — bolt.Open n'est joué que
// sur le fichier du test.

import (
	"os"
	"path/filepath"
	"testing"

	bolt "go.etcd.io/bbolt"
)

// homeAvecBase pose un AJEAN_HOME éphémère contenant les bases nommées, et
// garnit la première comme une vraie installation (une base bbolt réelle, pas un
// fichier vide : un fichier vide ne prouverait rien, bolt.Open l'accepterait).
func homeAvecBase(t *testing.T, noms ...string) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("AJEAN_HOME", home)
	for _, n := range noms {
		if err := ecrireBaseBolt(t, filepath.Join(home, n)); err != nil {
			t.Fatalf("poser %s : %v", n, err)
		}
	}
	return home
}

// ecrireBaseBolt crée une vraie base bbolt avec un bucket et une entrée, pour
// qu'elle ne soit pas confondue avec un fichier créé par erreur.
func ecrireBaseBolt(t *testing.T, chemin string) error {
	t.Helper()
	db, err := bolt.Open(chemin, 0o600, nil)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("config"))
		if err != nil {
			return err
		}
		return b.Put([]byte("marqueur"), []byte("données à ne pas perdre"))
	})
}

// TestBaseNommeeBddtchat : sur une installation neuve, la base s'appelle du nom
// d'aujourd'hui. Sans ce test, le renommage pourrait ne jamais avoir lieu et
// personne ne le verrait — le code fonctionnerait quand même.
func TestBaseNommeeBddtchat(t *testing.T) {
	home := homeAvecBase(t) // aucune base
	got := dbPath()
	voulu := filepath.Join(home, "bddtchat.db")
	if got != voulu {
		t.Fatalf("installation neuve : obtenu %q, voulu %q", got, voulu)
	}
	if got == filepath.Join(home, "ajean.db") {
		t.Fatal("la base cherche toujours « ajean.db » : le renommage n'a rien fait")
	}
}

// TestBaseAdmetAncienNom : LE cas qui fait la valeur du garde-fou. L'application
// d'une machine installée avant le renommage a sa base sous le nom d'avant. Sans
// admission, bolt.Open créerait un bddtchat.db VIDE à côté, et l'humain verrait
// une application toute neuve — sans qu'aucune erreur soit jamais levée.
func TestBaseAdmetAncienNom(t *testing.T) {
	home := homeAvecBase(t, "ajean.db")
	got := dbPath()
	if voulu := filepath.Join(home, "ajean.db"); got != voulu {
		t.Fatalf("machine installée avant le renommage : obtenu %q, voulu %q — 536 Mo de mémoire seraient abandonnés", got, voulu)
	}
	// Et surtout : le chemin désigné mène aux DONNÉES, pas à un fichier qui
	// sera créé à l'instant.
	if !isFile(got) {
		t.Fatalf("le chemin rendu %q ne mène à rien", got)
	}
	if isFile(filepath.Join(home, "bddtchat.db")) {
		t.Fatal("un bddtchat.db fantôme a été créé : la base vide fait redémarrer l'application sans mémoire")
	}
}

// TestBaseNouvelleGagneApresBascule : renommer l'ancien en nouveau EST le geste
// de bascule, et il doit suffire seul. L'ancien peut disparaître, rester, ou
// être archivé à côté : le nouveau compte dès qu'il existe.
func TestBaseNouvelleGagneApresBascule(t *testing.T) {
	home := homeAvecBase(t, "ajean.db", "bddtchat.db")
	got := dbPath()
	if voulu := filepath.Join(home, "bddtchat.db"); got != voulu {
		t.Fatalf("les deux existent : obtenu %q, voulu %q", got, voulu)
	}
}

// TestBaseNeCreePasUneBaseVide : l'erreur à ne plus commettre, jouée pour de
// vrai sur bbolt. On ouvre la base résolue sur une machine qui n'a que
// l'ancien nom, et on relit ce qu'elle contient. Si dbPath() se trompait, cette
// lecture échouerait sur une base fraîchement créée.
func TestBaseNeCreePasUneBaseVide(t *testing.T) {
	homeAvecBase(t, "ajean.db")
	chemin := dbPath()

	db, err := bolt.Open(chemin, 0o600, nil)
	if err != nil {
		t.Fatalf("ouvrir %s : %v", chemin, err)
	}
	defer db.Close()

	trouve := ""
	if err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("config"))
		if b == nil {
			return nil
		}
		trouve = string(b.Get([]byte("marqueur")))
		return nil
	}); err != nil {
		t.Fatalf("lire %s : %v", chemin, err)
	}
	if trouve != "données à ne pas perdre" {
		t.Fatalf("la base ouverte ne contient pas les données de l'humain (obtenu %q) — l'application redémarrerait sur du vide", trouve)
	}
}

// TestBaseContreEpreuve : le filet doit rougir si l'admission de l'ancien nom
// est retirée. Jouée sur le comportement, pas sur une intention : sur une
// machine qui n'a QUE l'ancien nom, le chemin rendu ne doit surtout PAS être le
// nouveau — sinon les tests ci-dessus réussiraient pour une raison sans rapport.
func TestBaseContreEpreuve(t *testing.T) {
	home := homeAvecBase(t)
	// Aucune base : le nouveau nom est bien celui rendu, et il n'existe pas
	// encore (il sera créé à l'écriture).
	if got := dbPath(); got != filepath.Join(home, "bddtchat.db") {
		t.Fatalf("installation neuve : obtenu %q", got)
	}
	if isFile(dbPath()) {
		t.Fatal("dbPath() crée la base en la résolvant : elle doit être rendue, pas ouverte")
	}
	// L'ancien nom, posé, doit être admis : s'il ne l'était pas, le test
	// TestBaseAdmetAncienNom ne prouverait rien.
	if err := ecrireBaseBolt(t, filepath.Join(home, "ajean.db")); err != nil {
		t.Fatalf("poser l'ancienne base : %v", err)
	}
	if got := dbPath(); got != filepath.Join(home, "ajean.db") {
		t.Fatalf("l'ancienne base n'est pas admise (obtenu %q) : le garde-fou serait un vert qui ne vérifie rien", got)
	}
	// Le nom de la constante est bien celui de l'ordre, pas un reste.
	if nomBase != "bddtchat.db" {
		t.Fatalf("nomBase = %q, voulu %q", nomBase, "bddtchat.db")
	}
}

// TestBaseOuverteParAjeanHome : dbPath suit AJEAN_HOME. La base ne doit jamais
// se retrouver dans le dossier d'une AUTRE instance — c'est ainsi que deux
// installations se sont déjà marché sur les données (mesuré 17/11, les deux
// chemins de journal qui ne coïncidaient pas).
func TestBaseOuverteParAjeanHome(t *testing.T) {
	premier := homeAvecBase(t, "bddtchat.db")
	if got := dbPath(); got != filepath.Join(premier, "bddtchat.db") {
		t.Fatalf("AJEAN_HOME ignoré : obtenu %q", got)
	}
	second := homeAvecBase(t, "ajean.db")
	got := dbPath()
	if got != filepath.Join(second, "ajean.db") {
		t.Fatalf("la base ne suit pas le nouveau AJEAN_HOME : obtenu %q, voulu sous %q", got, second)
	}
	// Rien de la première instance ne traîne dans la seconde.
	if _, err := os.Stat(filepath.Join(second, "bddtchat.db")); err == nil {
		t.Fatal("une base de la première instance a fuié dans la seconde")
	}
}
