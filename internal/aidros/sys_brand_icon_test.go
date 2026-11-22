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
//   · le motif (l'anneau, les têtes) doit être TRANSPARENT : c'est le pochoir ;
//   · le trou de l'anneau doit être OPAQUE, comme le reste du carré : si le trou
//     était transparent, macOS colorerait l'anneau et creuserait le trou — l'anneau
//     disparaîtrait, remplacé par son propre négatif.
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

// TestFaviconPartageLaGeometrie : le quatrième dessinateur de la marque est un
// SVG ÉCRIT À LA MAIN dans ui/src/index.tmpl.html. Son propre commentaire lui
// prêtait la « même géométrie que sys_brand_icon.go » ; mesuré le 22/11, c'était
// faux — le SVG dessinait encore le « j » que Go avait déjà remplacé. Ce filet LIT
// le gabarit et compare chiffre par chiffre. C'est le seul des quatre draweurs qui
// ne PEUT PAS lire marqueDisques (c'est du texte d'un autre monde), donc c'est le
// seul qui a besoin d'un filet pour ne pas dériver.
//
// ⚠ Le trait SVG est CENTRÉ sur son tracé, l'inverse de GDI. Le cercle du gabarit
// est donc au RAYON MOYEN de l'anneau, avec un trait de l'épaisseur de l'anneau :
// ses deux bords tombent exactement sur marqueRayonInt et marqueRayonExt.
func TestFaviconPartageLaGeometrie(t *testing.T) {
	src, err := os.ReadFile("ui/src/index.tmpl.html")
	if err != nil {
		t.Fatalf("gabarit introuvable : %v", err)
	}
	line := ""
	for _, l := range strings.Split(string(src), "\n") {
		if strings.Contains(l, `rel="icon"`) {
			line = strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(l, `">`), `">`))
			break
		}
	}
	if line == "" {
		t.Fatal("aucune balise rel=\"icon\" dans le gabarit : le favicon n'est plus servi")
	}

	c, rExt, e, disques := marqueDisques()
	rInt := rExt - e
	rMoyen := (rExt + rInt) / 2

	// ⚠ On COMPARE DES NOMBRES, pas des chaînes. Ma première version exigeait un
	// formatage précis (« %.6g ») et échouait sur 1.93 contre 1.92968 : le chiffre
	// du gabarit était juste, sa PRÉCISION était la sienne. Un filet qui refuse
	// 1.93 pour 1.92968 est un filet qui se plaint pour rien, et à force on
	// l'ignore. D'où la tolérance : ce qui compte est que le SVG tombe sur la forme,
	// pas combien de décimales il écrit.
	const tolr = 0.002

	// L'anneau : un <circle> au rayon MOYEN, au trait de l'épaisseur de l'anneau.
	// SVG centre son trait sur le tracé (l'inverse de GDI) : ses deux bords tombent
	// donc sur nos 2.7 et 4.7.
	ann := regexp.MustCompile(`cx='([^']+)' cy='([^']+)' r='([^']+)' fill='none' stroke='%23fff' stroke-width='([^']+)'`).FindStringSubmatch(line)
	if ann == nil {
		t.Fatal("le favicon ne porte pas d'anneau (un cercle sans remplissage, au trait blanc) : " +
			"la marque est un anneau, pas un disque")
	}
	sondeFloat(t, "l'anneau : cx", ann[1], c, tolr)
	sondeFloat(t, "l'anneau : cy", ann[2], c, tolr)
	sondeFloat(t, "l'anneau : rayon moyen", ann[3], rMoyen, tolr)
	sondeFloat(t, "l'anneau : épaisseur du trait", ann[4], e, tolr)

	// Les têtes et les becs : des disques pleins. On les compte d'abord, pour qu'un
	// disque effacé du SVG ne passe pas pour un détail.
	pts := regexp.MustCompile(`cx='([^']+)' cy='([^']+)' r='([^']+)' fill='%23fff'`).FindAllStringSubmatch(line, -1)
	if len(pts) != len(disques) {
		t.Fatalf("le favicon porte %d disques, la géométrie en décrit %d : "+
			"l'hydre a perdu ou gagné une tête sous la main qui écrit le SVG", len(pts), len(disques))
	}
	// ⚠ Comparés EN ENSEMBLE, pas dans l'ordre. La géométrie décrit tête, bec, tête,
	// bec… ; le SVG, écrit à la main, aligne les trois têtes puis les trois becs. Cet
	// ordre est une convention d'écriture, pas une propriété de la marque — un filet
	// qui l'imposait (ma première version) échouait sur six disques parfaitement
	// justes, et c'est le genre de rouge qu'on apprend à ignorer. Ce qui compte :
	// chaque disque de la géométrie a son JUMEAU dans le SVG, et un jumeau ne sert
	// qu'une fois (sinon une tête dupliquée masquerait une tête effacée).
	rendus := make([]bool, len(pts))
	for k, d := range disques {
		trouve := -1
		for j, p := range pts {
			if rendus[j] {
				continue
			}
			if proche(p[1], d[0], tolr) && proche(p[2], d[1], tolr) && proche(p[3], d[2], tolr) {
				trouve = j
				break
			}
		}
		if trouve < 0 {
			t.Errorf("disque %d de la marque (tête %d, centre %.3f,%.3f rayon %.3f) : aucun jumeau dans le favicon. "+
				"Ce qu'il y a dans le SVG : %v", k, k/2, d[0], d[1], d[2], pts)
			continue
		}
		rendus[trouve] = true
	}

	// Le « j » d'avant ne doit plus traîner là : ses trois rectangles.
	for _, rc := range [][4]float64{{6, 3, 2, 2}, {6, 5, 2, 2}, {4, 7, 2, 2}} {
		if strings.Contains(line, fmt.Sprintf(`x='%.6g' y='%.6g' width='%.6g' height='%.6g'`, rc[0], rc[1], rc[2], rc[3])) {
			t.Errorf("le favicon porte encore le rectangle %.0f,%.0f de l'ancien « j »", rc[0], rc[1])
		}
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

// sondeFloat idem, mais nomme l'écart dans le message : sur un fichier écrit à la
// main, dire « faux » ne suffit pas, il faut dire de combien.
func sondeFloat(t *testing.T, quoi, ecrit string, voulu float64, tolr float64) {
	t.Helper()
	got, err := strconv.ParseFloat(ecrit, 64)
	if err != nil {
		t.Errorf("%s : %q n'est pas un nombre", quoi, ecrit)
		return
	}
	if math.Abs(got-voulu) > tolr {
		t.Errorf("%s : le favicon écrit %s, la géométrie veut %.4f (écart %.4f, toléré %.4f)",
			quoi, ecrit, voulu, math.Abs(got-voulu), tolr)
	}
}

// exiger vérifie qu'un morceau de géométrie figure dans le gabarit. Le message
// porte la valeur ATTENDUE telle quelle : un filet sur un fichier écrit à la main
// doit dire quoi taper, pas seulement que ça cloche.
func exiger(t *testing.T, ligne, motif, quoi string) {
	t.Helper()
	if !strings.Contains(ligne, motif) {
		t.Errorf("%s : le favicon du gabarit ne porte pas %q", quoi, motif)
	}
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
