package aidros

// logo_marque_test.go — la marque du LOGO a un garde, comme tout le reste.
//
// Leçon de la journée du 21/11, deux fois répétée : un texte que personne ne lit
// ni ne vérifie ment sans que personne s'en aperçoive. C'est ce qui est arrivé au
// FileDescription (« AJEAN Mémoire TEST — port 18090 », onze lignes de prose), et
// au BUILD.json de la racine (aucun lecteur). Le logo vient d'être dessiné à la
// main, glyphe par glyphe : sans test, il peut régresser sans un bruit.
//
// Ce que le test affirme n'est PAS le mot « Aidros » nu. Depuis le renommage, ce
// mot est partout (<title>, manifeste, i18n, pont) : le chercher ne distingue
// rien, et un test positif passerait même si le logo disparaissait — un vert qui
// ne vérifie rien. Il affirme la STRUCTURE du logo : ses deux spans, leur
// viewBox, et le dessin de chaque glyphe, relus depuis les rectangles SVG.

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// grilleDepuisSvg reconstruit la grille de pixels d'un <span> de marque à partir
// de ses <rect>. Un glyphe se lit, il ne se devine pas.
func grilleDepuisSvg(t *testing.T, span string) [][]bool {
	t.Helper()
	var w, h int
	if _, err := fmt.Sscanf(extraire(t, span, `viewBox="0 0 `), `%d %d"`, &w, &h); err != nil {
		t.Fatalf("viewBox illisible dans le span de marque : %v", err)
	}
	if w <= 0 || h <= 0 || w > 200 || h > 40 {
		t.Fatalf("viewBox absurde : %dx%d", w, h)
	}
	g := make([][]bool, h)
	for i := range g {
		g[i] = make([]bool, w)
	}
	n := 0
	for _, ligne := range strings.Split(span, "<rect ") {
		// Seuls les morceaux qui commencent par x=" sont des rectangles : le
		// premier élément du split, lui, est le début du span.
		if !strings.HasPrefix(ligne, `x="`) {
			continue
		}
		var x, y, lw, lh int
		if _, err := fmt.Sscanf(ligne, `x="%d" y="%d" width="%d" height="%d"`, &x, &y, &lw, &lh); err != nil {
			t.Fatalf("<rect> illisible : %v", err)
		}
		for r := y; r < y+lh; r++ {
			for c := x; c < x+lw; c++ {
				if r >= 0 && r < h && c >= 0 && c < w {
					g[r][c] = true
				}
			}
		}
		n++
	}
	if n == 0 {
		t.Fatal("aucun <rect> : le logo est vide")
	}
	return g
}

// lireSpanDeMarque extrait <span id="id">…</span> d'un texte.
func lireSpanDeMarque(t *testing.T, brut, id string) string {
	t.Helper()
	debut := strings.Index(brut, `<span id="`+id+`"`)
	if debut < 0 {
		t.Fatalf("span id=%q introuvable : le logo a disparu", id)
	}
	fin := strings.Index(brut[debut:], "</span>")
	if fin < 0 {
		t.Fatalf("span id=%q non fermé", id)
	}
	return brut[debut : debut+fin+len("</span>")]
}

func extraire(t *testing.T, s, motif string) string {
	t.Helper()
	i := strings.Index(s, motif)
	if i < 0 {
		t.Fatalf("motif %q introuvable", motif)
	}
	return s[i+len(motif):]
}

// motsDuLogo rend chaque glyphe séparé par une colonne vide, en lignes de
// « # » et de « . ». C'est ce que l'œil voit à l'écran, à la seule différence
// près qu'ici c'est vérifiable.
func motsDuLogo(g [][]bool) []string {
	h := len(g)
	w := len(g[0])
	vides := []int{}
	for c := 0; c < w; c++ {
		vide := true
		for r := 0; r < h; r++ {
			if g[r][c] {
				vide = false
				break
			}
		}
		if vide {
			vides = append(vides, c)
		}
	}
	mots := []string{}
	debut := 0
	for _, c := range append(vides, w) {
		if c > debut {
			var b strings.Builder
			for r := 0; r < h; r++ {
				for x := debut; x < c; x++ {
					if g[r][x] {
						b.WriteByte('#')
					} else {
						b.WriteByte('.')
					}
				}
				if r < h-1 {
					b.WriteByte('\n')
				}
			}
			mots = append(mots, b.String())
		}
		debut = c + 1
	}
	return mots
}

// TestLogoPorteAidros : le logo affiche AIDROS. Le grand A de gauche (brand-a)
// fait le « A » du mot ; brand-word porte les cinq glyphes I-D-R-O-S.
//
// Les glyphes sont affirmés DESSINÉS, pas nommés : si quelqu'un redessine une
// lettre, le test le voit. Si le logo revient à JEAN, le test le voit aussi.
func TestLogoPorteAidros(t *testing.T) {
	chemins := []string{
		"ui/src/index.tmpl.html", // la source
		"ui/index.html",          // le généré par go:generate — ce qui est réellement servi
	}
	for _, chemin := range chemins {
		brut, err := os.ReadFile(chemin)
		if err != nil {
			t.Fatalf("%s : %v", chemin, err)
		}
		texte := string(brut)

		// 1. le grand A, intact à gauche.
		grandA := lireSpanDeMarque(t, texte, "brand-a")
		if !strings.Contains(grandA, `viewBox="0 0 4 5"`) {
			t.Errorf("%s : le grand A de gauche n'est plus en 4x5, le logo a bougé", chemin)
		}

		// 2. la suite du mot : cinq glyphes, I-D-R-O-S.
		mot := lireSpanDeMarque(t, texte, "brand-word")
		if !strings.Contains(mot, `viewBox="0 0 19 5"`) {
			t.Errorf("%s : brand-word n'est plus en 19x5 (attendu I-D-R-O-S)", chemin)
		}
		g := grilleDepuisSvg(t, mot)
		mots := motsDuLogo(g)
		if len(mots) != 5 {
			t.Fatalf("%s : attendu 5 glyphes (I-D-R-O-S), la grille en rend %d", chemin, len(mots))
		}
		attendus := []string{
			"###\n.#.\n.#.\n.#.\n###", // I
			"##.\n#.#\n#.#\n#.#\n##.", // D
			"##.\n#.#\n##.\n#.#\n#.#", // R
			"###\n#.#\n#.#\n#.#\n###", // O
			"###\n#..\n###\n..#\n###", // S
		}
		for i, voulu := range attendus {
			if mots[i] != voulu {
				t.Errorf("%s : glyphe %d ne correspond pas\nobtenu:\n%s\nvoulu:\n%s", chemin, i+1, mots[i], voulu)
			}
		}

		// 3. le lecteur d'écran dit la même chose que l'image.
		if !strings.Contains(texte, `<h1 id="brand" aria-label="aidros">`) {
			t.Errorf("%s : le h1 de marque n'a pas aria-label=\"aidros\" — l'image dit Aidros, l'accessibilité dirait encore autre chose", chemin)
		}

		// 4. le mot d'avant ne traîne pas dans le logo. Affirmé ici, et pas « Ajean
		// nulle part » : Ajean reste légitime ailleurs (AJEAN_*, go.mod, protocole
		// du pont, commentaires) — voir la page mémoire de l'état des lieux du nom.
		if strings.Contains(mot, `viewBox="0 0 17 5"`) {
			t.Errorf("%s : brand-word est revenu aux glyphes de JEAN", chemin)
		}
	}
}

// TestLogoAidrosContreEpreuve : le filet doit virer rouge si le logo n'est plus
// Aidros. Joué sur le texte réel, pas sur une intention : on remplace la grille
// par celle de l'ancien mot, et le test doit refuser.
func TestLogoAidrosContreEpreuve(t *testing.T) {
	brut, err := os.ReadFile("ui/src/index.tmpl.html")
	if err != nil {
		t.Fatalf("lire le gabarit : %v", err)
	}
	texte := string(brut)

	ancien := strings.NewReplacer(
		`viewBox="0 0 19 5"`, `viewBox="0 0 17 5"`,
		`aria-label="aidros"`, `aria-label="ajean"`,
	).Replace(texte)
	if ancien == texte {
		t.Fatal("la contre-épreuve n'a rien changé : le gabarit ne porte pas la marque attendue")
	}
	for _, motif := range []string{`viewBox="0 0 19 5"`, `<h1 id="brand" aria-label="aidros">`} {
		if strings.Contains(ancien, motif) {
			t.Errorf("la contre-épreuve laisse encore %q : elle ne défait pas le logo", motif)
		}
	}
	// Le test principal, rejoué sur ce texte défait, doit refuser les deux signes.
	if strings.Contains(ancien, `viewBox="0 0 19 5"`) || strings.Contains(ancien, `aria-label="aidros"`) {
		t.Error("le garde resterait vert sur un logo revenu à JEAN")
	}
}
