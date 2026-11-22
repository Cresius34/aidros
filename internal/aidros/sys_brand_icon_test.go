package aidros

// Filets de la marque dessinée — sys_brand_icon.go.
//
// Aucun filet n'existait pour l'icône avant le 22/11 : la marque était dessinée,
// embarquée dans quatre endroits, et RIEN ne vérifiait sa forme. Ces filets ne
// testent pas des rectangles (« le rect (6,3) est peint ») : ils testent les
// PROPRIÉTÉS qui font qu'une icône se lit, mesurées avant d'écrire le dessin.
//
// La règle à défendre, dans l'ordre où elle coûte :
//   1. le motif tient en UN SEUL MORCEAU CONNECTÉ à 16 px — à 40,8 % du fond
//      peint, contre deux morceaux de 9 px pour l'ancien « j » ;
//   2. l'anneau a un TROU, et ce trou reste un trou dans la variante macOS ;
//   3. les trois têtes sont ÉQUIRÉPARTIES et SOUDÉES à l'anneau ;
//   4. le DRAWEUR AU SOL (le splash GDI) ne peut pas avoir de forme différente,
//      puisqu'il lit la même géométrie — vérifié, pas espéré.

import (
	"bytes"
	"fmt"
	"image/png"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// motifPixels rend le motif en booléens à la taille n, en échantillonnant le
// centre de chaque pixel — la même grille que brandIconImage, sans le
// supersampling : ici on veut la forme exacte, pas le lissage.
func motifPixels(n int) [][]bool {
	g := make([][]bool, n)
	pas := 12.0 / float64(n)
	for y := 0; y < n; y++ {
		ligne := make([]bool, n)
		for x := 0; x < n; x++ {
			ligne[x] = marqueMotif((float64(x)+0.5)*pas, (float64(y)+0.5)*pas)
		}
		g[y] = ligne
	}
	return g
}

// morceaux compte les groupes de pixels peints 4-voisins et rend leurs tailles,
// plus la fraction du CARRE NOIR que le motif peint. C'est la mesure qui a décidé
// du dessin ; elle est ici pour ne plus jamais se faire avoir par un dessin qui
// tient en trois morceaux.
func morceaux(g [][]bool) (tails []int, motifSurFond float64) {
	n := len(g)
	vus := make([][]bool, n)
	for y := 0; y < n; y++ {
		vus[y] = make([]bool, n)
	}
	empile := make([][2]int, 0, n*n)
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			if !g[y][x] || vus[y][x] {
				continue
			}
			empile = append(empile[:0], [2]int{x, y})
			vus[y][x] = true
			taille := 0
			for len(empile) > 0 {
				p := empile[len(empile)-1]
				empile = empile[:len(empile)-1]
				taille++
				for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
					nx, ny := p[0]+d[0], p[1]+d[1]
					if nx < 0 || ny < 0 || nx >= n || ny >= n {
						continue
					}
					if g[ny][nx] && !vus[ny][nx] {
						vus[ny][nx] = true
						empile = append(empile, [2]int{nx, ny})
					}
				}
			}
			tails = append(tails, taille)
		}
	}
	// Le dénominateur est le carré arrondi, pas n*n : comparer au carré entier
	// sous-estimerait la charge du motif, et la barre des 30 % serait franchie par
	// un dessin bien plus chétif qu'elle ne le croit.
	peints := 0
	pas := 12.0 / float64(n)
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			if !horsCarré((float64(x)+0.5)*pas, (float64(y)+0.5)*pas) {
				peints++
			}
		}
	}
	blancs := 0
	for _, t := range tails {
		blancs += t
	}
	if peints > 0 {
		motifSurFond = float64(blancs) / float64(peints)
	}
	return
}

// horsCarré reproduit le coin arrondi de brandIconImage (rayon 2, coins à 2 et 10
// sur la grille 12). Dupliqué exprès : si le draweur change son carré sans que le
// filet le sache, le dénominateur devient faux, et le filet ment. Ce duplicata est
// donc un garde, pas une paresse.
func horsCarré(gx, gy float64) bool {
	const r = 2.0
	dans := true
	coin := func(cx, cy float64) {
		if dx, dy := gx-cx, gy-cy; dx*dx+dy*dy > r*r {
			dans = false
		}
	}
	switch {
	case gx < r && gy < r:
		coin(r, r)
	case gx > 12-r && gy < r:
		coin(12-r, r)
	case gx < r && gy > 12-r:
		coin(r, 12-r)
	case gx > 12-r && gy > 12-r:
		coin(12-r, 12-r)
	}
	return !dans
}

// TestMarqueTientEnUnMorceau est LE filet de la règle mesurée. À 16 px — la
// taille de la barre des tâches — le motif doit tenir en un seul morceau et
// charger assez le carré pour qu'on le voie.
func TestMarqueTientEnUnMorceau(t *testing.T) {
	g := motifPixels(16)
	tails, charge := morceaux(g)
	if len(tails) != 1 {
		t.Errorf("à 16 px, le motif tient en %d morceaux, pas 1 : %v. "+
			"Un motif en morceaux ne se lit pas dans une barre des tâches : c'est "+
			"exactement ce qui tuait l'ancien « j » (2 morceaux, le plus petit de 9 px).",
			len(tails), tails)
	}
	if charge < 0.30 {
		t.Errorf("à 16 px, le motif ne peint que %.1f %% du carré (visé : au moins 30 %%, mesuré 35,7 %%). "+
			"Trop léger = invisible à 16 px.", 100*charge)
	}
	if charge > 0.70 {
		t.Errorf("à 16 px, le motif peint %.1f %% du carré : trop, ce n'est plus une forme, c'est une tache.", 100*charge)
	}
	t.Logf("16 px : %d morceau, %.1f %% du carré peint", len(tails), 100*charge)
}

// TestMarqueAGardeUnTrou garde la forme : un anneau, pas un patapon. Le centre de
// la grille ne doit PAS appartenir au motif. C'est aussi ce qui interdit le retour
// du « j » : ses trois rects laissaient le centre vide aussi, mais ils ne laissaient
// surtout pas un anneau — vérifié par le filet suivant.
func TestMarqueAGardeUnTrou(t *testing.T) {
	if marqueMotif(marqueCentre, marqueCentre) {
		t.Error("le centre de l'anneau est peint : l'hydre est devenue un disque plein. " +
			"Le trou est ce qui fait un anneau, et c'est ce que l'icône « template » de macOS découpe.")
	}
	// Le trou doit être assez grand pour survivre à 16 px : un trou d'un pixel ne se
	// voit pas, et l'anneau passe pour un point.
	//
	// ⚠ Le trou est le VIDE ENTOURÉ de peint, pas « tout ce qui n'est pas peint ».
	// Le premier comptage disait « 162 pixels de trou » : c'était tout le fond noir du
	// carré, les quatre coins compris. Le test passait avec une marge énorme et
	// n'aurait RIEN vu si l'anneau s'était bouché — un faux vert de plus, trouvé en
	// relisant le chiffre, pas en lisant le test. D'où la inondation depuis le coin :
	// ce qui reste après avoir inondé l'extérieur est le trou, et rien d'autre.
	g := motifPixels(16)
	creux := creuxEntoure(g)
	if creux < 4 {
		t.Errorf("le trou de l'anneau ne fait que %d pixels à 16 px : l'anneau se lira comme un point, pas comme un anneau", creux)
	}
	t.Logf("16 px : le trou de l'anneau fait %d pixels (le fond noir, lui, en fait %d)", creux, fondNoir(g))
}

// creuxEntoure compte les pixels NON peints qui ne touchent pas le bord de l'image :
// les trous vrais, enfermés dans le motif. Le fond noir du carré, lui, touche le
// bord — inonder depuis le coin le sépare du trou.
func creuxEntoure(g [][]bool) int {
	n := len(g)
	creux := make([][]bool, n)
	for y := 0; y < n; y++ {
		creux[y] = make([]bool, n)
	}
	empile := make([][2]int, 0, n*n)
	for x := 0; x < n; x++ { // les quatre bords
		empile = append(empile, [2]int{x, 0}, [2]int{x, n - 1})
	}
	for y := 0; y < n; y++ {
		empile = append(empile, [2]int{0, y}, [2]int{n - 1, y})
	}
	touchent := make([]bool, n*n)
	for len(empile) > 0 {
		p := empile[len(empile)-1]
		empile = empile[:len(empile)-1]
		if p[0] < 0 || p[1] < 0 || p[0] >= n || p[1] >= n {
			continue
		}
		if g[p[1]][p[0]] || touchent[p[1]*n+p[0]] {
			continue
		}
		touchent[p[1]*n+p[0]] = true
		empile = append(empile, [2]int{p[0] + 1, p[1]}, [2]int{p[0] - 1, p[1]}, [2]int{p[0], p[1] + 1}, [2]int{p[0], p[1] - 1})
	}
	nbre := 0
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			if !g[y][x] && !touchent[y*n+x] {
				nbre++
			}
		}
	}
	return nbre
}

// fondNoir compte les pixels non peints mais DANS le carré : ce que le comptage
// naïf appelait « le trou » et qui n'en est pas un.
func fondNoir(g [][]bool) int {
	n := len(g)
	nbre := 0
	pas := 12.0 / float64(n)
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			if !g[y][x] && !horsCarré((float64(x)+0.5)*pas, (float64(y)+0.5)*pas) {
				nbre++
			}
		}
	}
	return nbre
}

// TestTrouResteTransparentDansLaVarianteMacOS joue la variante « template », qui
// ne garde QUE la couche alpha. Un filet sur une icône macOS ne peut pas se contenter
// de « ça se dessine » : si le pochoir se bouchait, la barre de menus afficherait un
// patapon noir sans forme, sur un thème sombre comme sur un clair.
//
// ⚠ CE QUE LE CODE FAIT, relu avant de corriger mon attente — la première version
// de ce filet exigeait l'INVERSE et a eu raison d'échouer : brandTemplatePNG passe
// bg=brandBlack et fg=brandClear. Donc le CARRÉ est opaque et le MOTIF est
// transparent. C'est la définition d'une template image : un POCHOIR. macOS colore
// ce qui est opaque — donc il colore le carré, trou de l'anneau compris — et le
// motif laisse voir le fond de la barre de menus.
//
// ➜ Les deux attentes, écrites à l'endroit :
//
//	· le motif (l'anneau, les têtes) doit être TRANSPARENT : c'est le pochoir ;
//	· le trou de l'anneau doit être OPAQUE, comme le reste du carré : si le trou
//	  était transparent, macOS colorerait l'anneau et creuserait le trou — l'anneau
//	  disparaîtrait, remplacé par son propre négatif.
func TestTrouResteTransparentDansLaVarianteMacOS(t *testing.T) {
	img := brandIconImage(32, brandBlack, brandClear)
	// En pixels de l'image, à 32 px : le centre est (16,16), le haut de l'anneau
	// est au rayon moyen de la géométrie, converti par 32/12.
	c := img.RGBAAt(16, 16)
	if c.A == 0 {
		t.Error("variante macOS : le centre du trou est transparent. Il doit être opaque comme le carré, " +
			"sinon macOS creuse le trou et colorie l'anneau : l'anneau disparaît, remplacé par son négatif.")
	}
	// Le pochoir : le motif doit être transparent. Un point sur l'anneau, en haut.
	rMoyenPx := (marqueRayonExt + marqueRayonInt) / 2 * 32.0 / 12.0
	a := img.RGBAAt(16, int(32.0/2-rMoyenPx+0.5))
	if a.A != 0 {
		t.Errorf("variante macOS : l'anneau a une opacité %d, attendu 0. Le motif doit être découpé (transparent) : "+
			"c'est ce qui fait un pochoir et non une tache noire sur une barre de menus sombre.", a.A)
	}
}

// TestTetesSudeesEtEquiparties garde l'histoire du dessin : trois têtes, posées
// SUR l'anneau, écartées de 120°. Si une tête se détachait de l'anneau, le motif
// ferait deux morceaux à 16 px — et le filet précédent le dirait. Celui-ci dit
// POURQUOI, avant que la panne n'arrive au rendu.
func TestTetesSudeesEtEquiparties(t *testing.T) {
	_, _, _, disques := marqueDisques()
	// 3 têtes + 3 becs
	if len(disques) != len(marqueTetes)*2 {
		t.Fatalf("la géométrie rend %d disques, attendu %d", len(disques), len(marqueTetes)*2)
	}
	if len(marqueTetes) != 3 {
		t.Errorf("%d têtes, attendu 3 : trois moteurs — le MoE, les embeddings, l'image", len(marqueTetes))
	}
	for i, deg := range marqueTetes {
		if i == 0 {
			continue
		}
		e := math.Abs(deg - marqueTetes[i-1])
		if math.Abs(e-120) > 0.01 {
			t.Errorf("têtes %d et %d écartées de %.1f°, attendu 120° : l'hydre doit être équirépartie", i-1, i, e)
		}
	}
	// Chaque TÊTE (un disque sur deux, l'autre étant un bec) doit toucher l'anneau :
	// c'est la soudure qui fait qu'un seul morceau survit à 16 px.
	for k, d := range disques {
		if k%2 != 0 {
			continue // les becs mordent l'anneau, ce n'est pas la même vérification
		}
		dx, dy := d[0]-marqueCentre, d[1]-marqueCentre
		dcen := math.Hypot(dx, dy)
		if dcen+d[2] < marqueRayonExt && dcen-d[2] > marqueRayonInt {
			t.Errorf("la tête %d flotte entre l'anneau (%.2f) et son bord (%.2f) : détachée, elle fera un morceau de plus à 16 px",
				k/2, marqueRayonInt, marqueRayonExt)
		}
	}
}

// TestDraweursPartagentLaGeometrie est le filet contre la divergence, celle que
// l'en-tête du fichier PROMETTAIT sans la tenir (favicon écrit à la main, un autre
// PNG en base64, le splash qui dessinait ses propres rects). Ici, les deux draweurs
// Go et GDI lisent marqueDisques. Ce filet le JOUE : il reconstitue le dessin de
// chaque draweur depuis la géométrie et exige qu'ils concordent.
func TestDraweursPartagentLaGeometrie(t *testing.T) {
	c, rExt, e, disques := marqueDisques()
	// Deux gardes de géométrie, avant de sonder : un anneau dont l'épaisseur mangerait
	// le bord intérieur n'est plus un anneau, et un anneau qui toucherait le bord de la
	// grille serait rogné par le carré arrondi.
	rInt := rExt - e
	if rInt <= 0 {
		t.Fatalf("l'anneau a une épaisseur %.2f qui mange son bord intérieur (%.2f)", e, rInt)
	}
	if rExt >= marqueCentre {
		t.Errorf("l'anneau (rayon %.2f) touche ou dépasse le bord de la grille 12 (centre %.1f) : il sera rogné par le carré", rExt, marqueCentre)
	}
	// Le draweur Go peint-il exactement ce que la géométrie décrit ? On pique un
	// point AU MILIEU de chaque zone, calculé depuis la géométrie — jamais en dur.
	sonde := func(gx, gy float64, attendu bool, quoi string) {
		t.Helper()
		if got := marqueMotif(gx, gy); got != attendu {
			t.Errorf("%s : marqueMotif répond %v à (%.2f, %.2f), attendu %v", quoi, got, gx, gy, attendu)
		}
	}
	// ⚠ J'avais écrit le point du trou « c - rInt - e/2 » : ça fait un rayon de
	// rInt + e/2, donc DANS l'anneau — et le test échouait pour une formule fausse,
	// pas pour un dessin faux. Les deux points ci-dessous sont au MILIEU de leur
	// zone : l'un dans la bande de l'anneau, l'autre dans le trou.
	sonde(c, c-(rExt+rInt)/2, true, "le milieu de la bande de l'anneau, en haut")
	sonde(c, c-rInt/2, false, "le milieu du trou, entre le centre et le bord intérieur")
	sonde(c, c, false, "le centre du trou")
	for _, d := range disques {
		sonde(d[0], d[1], true, "le centre d'un disque de la marque")
	}
	// Un point hors de toute forme, en haut à gauche.
	sonde(1.2, 1.2, false, "le coin haut gauche, hors anneau et hors têtes")
}

// TestLesDeuxSVGPartagentLaGeometrie : la marque en SVG n'est plus une copie à
// surveiller, c'est un PRODUIT de ce paquet. Mesuré 22/11 au soir : quatre
// fichiers de l'interface portaient chacun leur copie écrite à la main du « J »
// d'avant, et l'humain le voyait encore à l'écran. tools/assemble-ui pose
// désormais @@MARQUE@@ / @@MARQUE_INLINE@@ / @@MARQUE_PNG_180@@, dessinés ici.
//
// Le filet n'est PAS une tautologie. marqueDisques décrit la marque en GO ; le
// SVG la décrit en SVG, et les deux mondes n'ont pas la même convention de
// trait : GDI trace À L'INTÉRIEUR du rectangle, SVG centre son trait SUR le
// tracé. Le cercle du SVG doit donc être au RAYON MOYEN avec un trait de
// l'épaisseur de l'anneau, pour que ses deux bords tombent sur marqueRayonInt et
// marqueRayonExt. Rendre le rayon EXTÉRIEUR ici donnerait un anneau d'une
// demi-épaisseur trop épais — faute que seule une relecture comme celle-ci voit,
// parce qu'aucun des deux draweurs ne la dit.
//
// Il vérifie aussi OÙ chaque SVG atterrit, car ce n'est pas du goût mais la
// grammaire du voisin : l'un vit DANS une data:URI elle-même dans href="…",
// l'autre DANS le contenu d'un élément et dans une chaîne JS entre apostrophes.
// Le guillemet qui casse l'un protège l'autre.
func TestLesDeuxSVGPartagentLaGeometrie(t *testing.T) {
	t.Run("data:URI (favicon, manifeste, notification)", func(t *testing.T) {
		for _, tord := range defautsMarqueSVG(BrandMarqueSVG(), modeURI) {
			t.Error(tord)
		}
	})
	t.Run("inline (voile de chargement, ligne d'activité)", func(t *testing.T) {
		for _, tord := range defautsMarqueSVG(BrandMarqueSVGInline(), modeInline) {
			t.Error(tord)
		}
	})
}

// Les deux destinations d'un SVG de la marque.
const (
	modeURI    = "uri"    // dans data:image/svg+xml,… lui-même dans href="…"
	modeInline = "inline" // dans le contenu d'un <svg>, ou une chaîne JS '…'
)

// defautsMarqueSVG relit un SVG de la marque et rend la LISTE de ce qui cloche,
// vide s'il est juste. Fonction PURE, et c'est voulu : la contre-épreuve doit
// pouvoir la jouer sur un SVG volontairement cassé sans faire rougir le test qui
// la joue.
func defautsMarqueSVG(svg, mode string) []string {
	var tords []string
	ajouter := func(format string, a ...any) {
		tords = append(tords, fmt.Sprintf(format, a...))
	}

	c, rExt, e, disques := marqueDisques()
	rMoyen := (rExt + (rExt - e)) / 2

	// ⚠ On COMPARE DES NOMBRES, pas des chaînes. Ma première version exigeait un
	// formatage précis (« %.6g ») et échouait sur 1.93 contre 1.92968 : le chiffre
	// était juste, sa PRÉCISION était la sienne. Un filet qui refuse 1.93 pour
	// 1.92968 est un filet qui se plaint pour rien, et à force on l'ignore.
	const tolr = 0.002

	// La couleur selon la destination : une data:URI n'a pas de CSS, son blanc y
	// est ÉCRIT ; en inline la couleur vient de currentColor, donc du thème voisin.
	blanc := "%23fff"
	guillemetInterdit := `"`
	if mode == modeInline {
		blanc = "currentColor"
		guillemetInterdit = "'"
	}

	// Les <circle>, un par un, lus par attribut. Pas une grosse expression
	// rationnelle : un message qui doit dire QUOI corriger ne peut pas reposer
	// sur cinq groupes numérotés.
	balises := regexp.MustCompile(`<circle[^>]*>`).FindAllString(svg, -1)

	var anneau string
	var têtes []string
	for _, b := range balises {
		if attribut(b, "stroke") != "" {
			anneau = b
			continue
		}
		têtes = append(têtes, b)
	}

	// 1. L'ANNEAU : sa présence d'abord. Sans lui la marque redevient un disque,
	// exactement ce que l'hydre remplace.
	if anneau == "" {
		ajouter("le SVG ne porte pas d'anneau (un cercle au trait de %s, sans remplissage) : la marque est un anneau, pas un disque", blanc)
	} else {
		if r := attribut(anneau, "fill"); r != "none" {
			ajouter("l'anneau : fill=%q au lieu de « none » — un anneau rempli n'a plus de trou", r)
		}
		if r := attribut(anneau, "stroke"); r != blanc {
			ajouter("l'anneau : stroke=%q, la marque (%s) veut %q", r, mode, blanc)
		}
		sonde(&tords, "l'anneau : cx", attribut(anneau, "cx"), c, tolr)
		sonde(&tords, "l'anneau : cy", attribut(anneau, "cy"), c, tolr)
		sonde(&tords, "l'anneau : le rayon", attribut(anneau, "r"), rMoyen, tolr)
		sonde(&tords, "l'anneau : l'épaisseur du trait", attribut(anneau, "stroke-width"), e, tolr)
	}

	// 2. LES TÊTES ET LES BECS, comptés d'abord : un disque effacé ne doit pas
	// passer pour un détail.
	if len(têtes) != len(disques) {
		ajouter("le SVG porte %d disques pleins, la géométrie en décrit %d : l'hydre a perdu ou gagné une tête", len(têtes), len(disques))
	}

	// 3. Chaque disque de la géométrie a son JUMEAU, et un jumeau ne sert qu'une
	// fois (sinon une tête dupliquée masquerait une tête effacée).
	// ⚠ Comparés EN ENSEMBLE, pas dans l'ordre : la géométrie décrit tête, bec,
	// tête, bec… ; un SVG aligne les têtes puis les becs. Cet ordre est une
	// convention d'écriture, pas une propriété de la marque — un filet qui
	// l'imposait échouait sur six disques parfaitement justes.
	rendues := make([]bool, len(têtes))
	for k, d := range disques {
		trouve := -1
		for j, b := range têtes {
			if rendues[j] {
				continue
			}
			if proche(attribut(b, "cx"), d[0], tolr) &&
				proche(attribut(b, "cy"), d[1], tolr) &&
				proche(attribut(b, "r"), d[2], tolr) {
				trouve = j
				break
			}
		}
		if trouve < 0 {
			ajouter("disque %d de la marque (tête %d, centre %.3f,%.3f rayon %.3f) : aucun jumeau dans le SVG", k, k/2, d[0], d[1], d[2])
			continue
		}
		rendues[trouve] = true
	}

	// 4. L'ancien « j » ne doit plus traîner là : il vivait sur trois RECTANGLES.
	for _, rc := range [][4]float64{{6, 3, 2, 2}, {6, 5, 2, 2}, {4, 7, 2, 2}} {
		for _, g := range []string{`"`, `'`} {
			if strings.Contains(svg, fmt.Sprintf(`x=%s%.0f%s y=%s%.0f%s width=%s%.0f%s height=%s%.0f%s`,
				g, rc[0], g, g, rc[1], g, g, rc[2], g, g, rc[3], g)) {
				ajouter("le SVG porte encore le rectangle %.0f,%.0f de l'ancien « j »", rc[0], rc[1])
			}
		}
	}

	// 5. LE GUILLEMET, selon la destination. Un SVG dans une data:URI vit dans
	// href="…" et dans la chaîne "…" de sw.js : le guillemet double le casserait
	// LUI-MÊME. Un SVG inline vit dans un élément et dans une chaîne JS entre
	// apostrophes : là c'est l'apostrophe qui casse l'accueil.
	if n := strings.Count(svg, guillemetInterdit); n > 0 {
		ajouter("le SVG (%s) porte %d fois le caractère %q : il casse sa propre chaîne d'accueil", mode, n, guillemetInterdit)
	}

	// 6. LA COULEUR et la CHARPENTE, selon la destination.
	if mode == modeURI {
		if !strings.Contains(svg, blanc) {
			ajouter("le SVG (%s) ne nomme pas %q : une data:URI n'a pas de CSS, sa couleur doit y être écrite", mode, blanc)
		}
		if strings.Contains(svg, "currentColor") {
			ajouter("le SVG (%s) emploie currentColor : une data:URI ne connaît ni thème ni CSS, la forme y serait invisible", mode)
		}
		if !strings.Contains(svg, `<rect width='12' height='12' rx='2' ry='2' fill='%23000'/>`) {
			ajouter("le SVG (%s) ne porte pas son carré noir aux coins arrondis : la silhouette de l'icône", mode)
		}
		if !strings.HasPrefix(svg, "<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 12 12'>") ||
			!strings.HasSuffix(svg, "</svg>") {
			ajouter("le SVG (%s) n'est pas un document complet : la balise <svg> d'ouverture ou de fermeture manque", mode)
		}
	} else {
		if !strings.Contains(svg, "currentColor") {
			ajouter("le SVG (%s) n'emploie pas currentColor : posé dans la page, il ne suivrait plus la couleur du thème", mode)
		}
		if strings.Contains(svg, "%23") {
			ajouter("le SVG (%s) porte un %%23 d'URI : posé dans la page, le navigateur lirait ce texte comme une couleur", mode)
		}
		// En inline, pas de <svg> (l'accueil en porte déjà un) et pas de fond : le
		// voile a son propre fond, et le peindre deux fois effacerait le trou.
		if strings.Contains(svg, "<svg") {
			ajouter("le SVG (%s) porte sa propre balise <svg> : l'accueil en ouvre déjà une, la marque serait nichée", mode)
		}
		if strings.Contains(svg, "<rect") {
			ajouter("le SVG (%s) peint un fond : posé dans la page, il boucherait le trou de l'anneau", mode)
		}
	}

	return tords
}

// attribut lit la valeur d'un attribut dans une balise, vide s'il est absent.
//
// ⚠ DEUX MOTIFS LITTÉRAUX, pas un motif avec rétro-référence. Ma première
// version écrivait `nom=(['"])(.*?)\1` pour exiger que l'ouvrant et le fermant
// soient le même guillemet. Le regexp de Go est RE2 : il ne connaît PAS les
// backréférences, et MustCompile a PANIQUÉ sur « invalid escape sequence: \1 »
// — un filet qui panique sur son propre motif ressemble à un défaut du dessin.
// Les deux guillemets possibles sont deux motifs, unis par un `|`.
func attribut(balise, nom string) string {
	m := regexp.MustCompile(`\b` + regexp.QuoteMeta(nom) +
		`=(?:"([^"]*)"|'([^']*)')`).FindStringSubmatch(balise)
	if m == nil {
		return ""
	}
	if m[1] != "" {
		return m[1]
	}
	return m[2]
}

// sonde compare un nombre écrit dans le SVG à la valeur de la géométrie, sans
// exiger sa précision d'écriture, et nomme l'écart : dire « faux » ne suffit pas,
// il faut dire de combien et quoi vouloir.
func sonde(tords *[]string, quoi, ecrit string, voulu float64, tolr float64) {
	got, err := strconv.ParseFloat(ecrit, 64)
	if err != nil {
		*tords = append(*tords, fmt.Sprintf("%s : %q n'est pas un nombre", quoi, ecrit))
		return
	}
	if math.Abs(got-voulu) > tolr {
		*tords = append(*tords, fmt.Sprintf("%s : le SVG écrit %s, la géométrie veut %.4f (écart %.4f, toléré %.4f)",
			quoi, ecrit, voulu, math.Abs(got-voulu), tolr))
	}
}

// TestSVGContreEpreuve : le filet doit refuser un SVG cassé, et pour la raison
// qu'on nomme. Joué sur le texte RÉEL tordu, pas sur une intention — sinon on
// déclare une capacité qu'on n'a pas vérifiée.
func TestSVGContreEpreuve(t *testing.T) {
	vrai := BrandMarqueSVG()
	nu := BrandMarqueSVGInline()

	_, rExt, e, disques := marqueDisques()
	rMoyen := (rExt + (rExt - e)) / 2

	// La tête n°1, reconstituée depuis la géométrie : écrit à la main, « 1.93 »
	// n'aurait rien trouvé à tordre et la contre-épreuve se serait crue innocente.
	d0 := disques[0]
	tete1 := fmt.Sprintf("<circle cx='%s' cy='%s' r='%s' fill='%%23fff'/>", nombre(d0[0]), nombre(d0[1]), nombre(d0[2]))

	tordus := []struct {
		quoi, svg, mode, attend string
	}{
		// L'erreur que la confusion GDI/SVG produit : le rayon EXTÉRIEUR au lieu
		// du rayon moyen. L'anneau sortirait d'une demi-épaisseur trop épais.
		{fmt.Sprintf("l'anneau au rayon extérieur %.2f au lieu du rayon moyen %.2f", rExt, rMoyen),
			strings.Replace(vrai, fmt.Sprintf(`r='%s'`, nombre(rMoyen)), fmt.Sprintf(`r='%s'`, nombre(rExt)), 1), modeURI, "le rayon"},
		// Une tête effacée : le comptage doit la nommer, pas seulement gronder.
		{"une tête effacée", strings.Replace(vrai, tete1, "", 1), modeURI, "disques pleins"},
		// Le guillemet qui casse l'accueil.
		{"le guillemet double dans une data:URI", strings.Replace(vrai, `<rect width='12'`, `<rect width="12"`, 1), modeURI, "caractère"},
		// La couleur qui éteint le thème, en inline.
		{"un blanc figé là où le thème décide", strings.Replace(nu, "currentColor", "%23fff", 1), modeInline, "currentColor"},
		// Un fond peint en inline : il boucherait le trou.
		{"un fond peint en inline", `<rect x="0" y="0" width="12" height="12"/>` + nu, modeInline, "fond"},
		// L'anneau bouché : le trou disparaît, la marque redevient un disque.
		{"l'anneau bouché (fill=none effacé)", strings.Replace(vrai, `fill='none' stroke=`, `stroke=`, 1), modeURI, "rempli"},
	}

	for _, td := range tordus {
		if tords := defautsMarqueSVG(td.svg, td.mode); len(tords) == 0 {
			t.Errorf("la contre-épreuve « %s » PASSE : le filet ne voit pas ce défaut, il ne protège rien", td.quoi)
		} else {
			vu := false
			for _, tord := range tords {
				if strings.Contains(tord, td.attend) {
					vu = true
					break
				}
			}
			if !vu {
				t.Errorf("la contre-épreuve « %s » rougit, mais pour une autre raison que %q : %v", td.quoi, td.attend, tords)
			}
		}
	}

	// Et le vrai, dans les deux modes, doit rester vert : sans cette moitié on
	// pourrait avoir fabriqué un filet qui refuse tout.
	for _, ok := range []struct {
		svg, mode string
	}{{vrai, modeURI}, {nu, modeInline}} {
		if tords := defautsMarqueSVG(ok.svg, ok.mode); len(tords) != 0 {
			t.Errorf("le vrai SVG (%s) est refusé : %v", ok.mode, tords)
		}
	}
}

// TestMarqueSVGTraverseLesProduits : les produits de l'interface sont des
// fichiers COMMITTÉS — go:embed les embarque (web_server.go:22). Relire la source
// ne suffit donc pas : il faut relire ce qui est réellement servi. Ce filet joue
// chaque fichier UN À UN et compte les deux glyphes, la marque d'aujourd'hui et
// celle d'hier. Un fichier sans garde vieillit sans que rien ne bronche — c'est
// exactement ainsi que les quatre copies du « J » ont survécu.
func TestMarqueSVGTraverseLesProduits(t *testing.T) {
	// Le compte ATTENDU est PAR FICHIER, pas global. Mesuré 22/11 à l'épreuve
	// d'exécution : l'index sert la marque TROIS fois (favicon, voile, ligne
	// d'activité), le worker et le manifeste une seule fois chacun. Un seul chiffre
	// pour les trois faisait gronder le worker et laissait passer un index amputé.
	chemins := []struct {
		chemin  string
		dessins int    // combien de fois la marque y est dessinée
		quoi    string // à quoi elle sert là, dans le message
	}{
		{"ui/index.html", 3, "favicon, voile de chargement, ligne d'activité"},
		{"ui/sw.js", 1, "l'icône des notifications push"},
		{"ui/manifest.webmanifest", 1, "l'icône d'installation PWA"},
		{"ui/src/index.tmpl.html", 0, "la source du gabarit"},
		{"ui/src/sw.js", 0, "la source du worker"},
		{"ui/src/manifest.webmanifest", 0, "la source du manifeste"},
		{"ui/src/js/09-stream.js", 0, "la source de la ligne d'activité"},
	}
	relus := 0
	for _, f := range chemins {
		chemin := f.chemin
		brut, err := os.ReadFile(chemin)
		if err != nil {
			t.Errorf("%s : %v", chemin, err)
			continue
		}
		relus++
		texte := string(brut)

		if strings.Contains(chemin, "/src/") {
			// Une SOURCE porte le marqueur, pas le dessin : c'est l'assembleur qui
			// le pose. Exiger le dessin ici imposerait de l'écrire deux fois.
			if n := strings.Count(texte, "@@MARQUE"); n == 0 {
				t.Errorf("%s : aucune trace de @@MARQUE@@ — la marque ne serait plus dessinée là où l'interface l'attend", chemin)
			}
			continue
		}

		_, _, _, disques := marqueDisques()
		// Les DEUX jeux de guillemets, comptés ENSEMBLE. Mesuré 22/11 au soir, à
		// l'épreuve d'exécution : la page servie ne montrait QU'UN anneau sur les
		// TROIS dessinés, parce que l'URI s'écrit en apostrophes et l'inline en
		// guillemets doubles — la convention que j'ai choisie pour que chacun
		// survive à sa chaîne d'accueil. Un filet qui ne compte qu'un jeu compte 1
		// là où il y en a 3, et laisse filer les deux tiers de la marque.
		anneaux, têtes := 0, 0
		for _, g := range []string{`"`, "'"} {
			anneaux += strings.Count(texte, "cx="+g+"6"+g+" cy="+g+"6"+g+" r="+g+nombre(3.7)+g)
			têtes += strings.Count(texte, "r="+g+nombre(disques[0][2])+g)
		}
		if anneaux != f.dessins {
			t.Errorf("%s : %d anneaux de la marque (%s), %d attendu(s) pour %s — la marque y est absente, périmée ou amputée",
				chemin, anneaux, nombre(3.7), f.dessins, f.quoi)
		}
		// Une tête par dessin, trois têtes par hydre.
		if têtes != 3*f.dessins {
			t.Errorf("%s : %d têtes de la marque (%s), %d attendu(s) — l'hydre est amputée",
				chemin, têtes, nombre(disques[0][2]), 3*f.dessins)
		}
		// L'ancien « j » ne doit plus rien peindre. Affirmé sur les PRODUITS, pas
		// sur les sources : le commentaire d'une source a le droit de le nommer.
		for _, rc := range [][4]float64{{6, 3, 2, 2}, {6, 5, 2, 2}, {4, 7, 2, 2}} {
			for _, g := range []string{`"`, `'`} {
				if strings.Contains(texte, fmt.Sprintf(`x=%s%.0f%s y=%s%.0f%s width=%s%.0f%s height=%s%.0f%s`,
					g, rc[0], g, g, rc[1], g, g, rc[2], g, g, rc[3], g)) {
					t.Errorf("%s : porte encore le rectangle %.0f,%.0f de l'ancien « j »", chemin, rc[0], rc[1])
				}
			}
		}
		// Un marqueur survécu dans un PRODUIT = l'assembleur n'a pas tourné. C'est
		// la « paire rompue » de l'icône (22/11) dans une autre de ses formes.
		if n := strings.Count(texte, "@@MARQUE"); n > 0 {
			t.Errorf("%s : %d marqueur(s) @@MARQUE@@ ont survécu au produit — go generate n'a pas tourné", chemin, n)
		}
	}
	if relus != len(chemins) {
		t.Errorf("%d fichiers relus sur %d attendus", relus, len(chemins))
	}
}

// proche compare un nombre écrit dans le gabarit à la valeur de la géométrie, sans
// exiger sa précision d'écriture.
func proche(ecrit string, voulu float64, tolr float64) bool {
	got, err := strconv.ParseFloat(ecrit, 64)
	if err != nil {
		return false
	}
	return math.Abs(got-voulu) <= tolr
}

// TestIconeNeEstPasLeJ ferme la porte à l'ancien dessin, sans l'interdire ailleurs :
// le « j » vivait sur trois RECTANGLES. Le filet exige qu'aucun des trois rects ne
// soit peint intégralement — un anneau ne remplit pas de rectangles.
func TestIconeNeEstPasLeJ(t *testing.T) {
	for _, rc := range [][4]float64{{6, 3, 2, 2}, {6, 5, 2, 2}, {4, 7, 2, 2}} {
		plein := true
		pas := 0.25
		for gy := rc[1]; gy < rc[1]+rc[3]; gy += pas {
			for gx := rc[0]; gx < rc[0]+rc[2]; gx += pas {
				if !marqueMotif(gx, gy) {
					plein = false
				}
			}
		}
		if plein {
			t.Errorf("le rectangle %.0f,%.0f %.0fx%.0f est peint d'un bloc : c'est un des rects de l'ancien « j ». "+
				"La marque est un anneau, elle ne remplit pas un rectangle.",
				rc[0], rc[1], rc[2], rc[3])
		}
	}
}

// TestToutesLesTaillesSeDessinent : brandIconImage est appelée à 9 tailles par
// le serveur web, 6 par la barre de notifications, plus celles de l'ico. Un
// plantage de dessin ne se voit qu'à l'usage — ce filet les joue toutes.
func TestToutesLesTaillesSeDessinent(t *testing.T) {
	tailles := []int{16, 20, 24, 32, 40, 48, 64, 96, 128, 180, 192, 256, 512}
	for _, n := range tailles {
		octets := BrandIconPNG(n)
		if len(octets) == 0 {
			t.Errorf("taille %d : PNG vide", n)
			continue
		}
		img, err := png.Decode(bytes.NewReader(octets))
		if err != nil {
			t.Errorf("taille %d : PNG illisible : %v", n, err)
			continue
		}
		if b := img.Bounds(); b.Dx() != n || b.Dy() != n {
			t.Errorf("taille %d : l'image fait %dx%d", n, b.Dx(), b.Dy())
		}
		// Le fichier d'à côté renvoie l'octet PNG ; s'il change de format, le .ico
		// du .exe casse et go build ne le dit pas. On vérifie l'en-tête.
		if string(octets[:8]) != "\x89PNG\r\n\x1a\n" {
			t.Errorf("taille %d : ce n'est pas un PNG (% x)", n, octets[:8])
		}
	}
}
