package aidros

// sys_brand_icon.go — la marque dessinée à la volée : l'hydre-ouroboros.
//
// LA FORME est expliquée plus bas, à marqueMotif, avec la mesure qui l'a choisie.
//
// ⚠ LA PROMOSSE D'UNICITÉ DE CE FICHIER ÉTAIT FAUSSE, mesurée le 22/11. Son
// en-tête affirmait « RÉPLIQUE EXACTE du favicon de l'UI web » et « une seule
// source pour tous les usages » ; il dessinait en fait, en toutes lettres, un
// « j » blanc que le favicon ne lui empruntait pas :
//   · le favicon de l'UI est un SVG ÉCRIT À LA MAIN dans index.tmpl.html:15, en
//     data:image/svg+xml, avec ses propres formes — il ne lit pas ce fichier ;
//   · l'apple-touch-icon (index.tmpl.html:16) est un AUTRE dessin encore : un PNG
//     180x180 en base64, relu DIFFÉRENT de cmd/aidros/icon.png du dépôt.
// Donc changer la marque, c'est TROIS éditions — ce fichier, le SVG du gabarit,
// le base64 du gabarit — plus le .syso régénéré (voir plus bas). Un commentaire
// qui promet l'unicité pendant que trois endroits dessinent chacun leur compte
// est pire que pas de commentaire : il endort.
//
// ⚠ LE MAILLON QUI SAUTE, mesuré le même jour. La chaîne est :
//
//     ce fichier —go:generate→ tools/gen-icon → cmd/aidros/icon.{ico,png}
//                —goversioninfo→ resource_windows_*.syso —go build→ l'icône DANS l'exe
//
// `goversioninfo` n'est PAS au PATH sur ce poste. Sur `go generate ./...` il
// échoue en disant « executable file not found in %PATH% », APRÈS que gen-icon a
// réussi. Résultat : icon.ico est régénéré, le .syso ne l'est pas — et c'est le
// .syso que go build embarque. On peut donc changer la marque, reconstruire, et
// l'exe affiche encore l'ancienne. Relu ce jour : le .syso datait du 21/11,
// l'ico du 22/11.
//
// ➜ Après tout changement de marque, jouer goversioninfo EN CHEMIN COMPLET
// (C:\Users\julie\go\bin\goversioninfo.exe) et RELIRE la date du .syso. Deux
// artefacts qui doivent coïncider, un seul régénéré : c'est la famille de défaut
// de la « paire rompue » BUILD.json ↔ exe (16/11), deux fois mordue déjà.
//
// Aucun asset binaire à committer pour le DESSIN : tout est tracé ici. Les deux
// fichiers sous cmd/aidros/ sont des artefacts produits, pas des sources — et ils
// ont d'ailleurs DÉJÉ DÉRVÉ : rejoués, ils sortent à 5 278 et 10 400 octets,
// quand le dépôt en porte 5 994 et 30 943 (mesuré deux fois, gen-icon est
// déterministe). Le plus gros n'est donc plus ce que l'outil produit.

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
)

const trayIconSize = 32

var (
	brandBlack = color.RGBA{0x00, 0x00, 0x00, 0xff}
	brandWhite = color.RGBA{0xff, 0xff, 0xff, 0xff}
	brandClear = color.RGBA{0, 0, 0, 0}
)

// ─── la marque : l'hydre-ouroboros ────────────────────────────────────────────
//
// Un anneau qui se mord la queue, et trois têtes soudées à cet anneau.
//
// POURQUOI CETTE FORME, et pas seulement un goût :
//
//	· l'anneau fermé qui se mord la queue, c'est la boucle de ce projet — générer,
//	  analyser, corriger, recommencer ;
//	· trois têtes, ce sont nos trois moteurs : le MoE, les embeddings, l'image.
//	  L'hydre repousse ce qu'on lui coupe ; éteindre un moteur n'éteint pas
//	  l'application ;
//	· et le « j » ne disait plus rien : c'était l'initiale d'AJEAN. Le fichier le
//	  disait lui-même, en toutes lettres : « carré à coins arrondis NOIR + « j »
//	  blanc ». Depuis le renommage en Aidros (21/11), plus rien dans l'icône ne
//	  pointait vers le nom du produit. Le « j » tenait sur trois rectangles,
//	  (6,3) (6,5) (4,7) sur la grille 12 — connu pour qui voudrait le rejouer.
//
// ⭐ CE QUE LA MESURE A DÉCIDÉ, pas mon intuition. Le critère est la taille de la
// barre des tâches Windows, 16 px. Compté sur une maquette jouée AVANT d'écrire
// (fraction du fond peint, nombre de morceaux DÉTACHÉS, taille du plus petit) :
//
//	motif                     16 px   fond peint   morceaux   plus petit
//	le « j » d'avant            16       11,8 %         2          9 px
//	anneau + une tête           16       43,1 %         1         91 px
//	anneau + trois têtes        16       40,8 %         1         86 px
//
// Je craignais que trois têtes ne survivent pas à 16 px. FAUX, et la raison vaut
// d'être écrite : les têtes sont SOUDÉES à l'anneau, donc le motif reste UN SEUL
// morceau connecté, et c'est l'anneau qui porte la forme. L'anneau est même plus
// robuste que le « j » qu'il remplace — un seul tenant contre deux morceaux
// détachés de 9 px.
//
// ➜ LA RÈGLE DE DESSIN qui en découle, à ne pas négocier : à 16 px on ne
// distingue pas trois têtes de trois encoches — la tête ne compte qu'à partir de
// 32 px. Donc UNE forme unique et forte à 16 px, son histoire à 128 px. Les têtes
// sont là pour qui regarde de près ; l'anneau est là pour qui a juste besoin de
// retrouver sa fenêtre dans la barre des tâches.
//
// ⚠ CE QUE LE CODE LIVRÉ REND, mesuré après coup (le tableau ci-dessus est la
// MAQUETTE, avec une géométrie légèrement différente — citer son 40,8 % comme s'il
// sortait d'ici serait un chiffre écrit sans mesurer) :
//
//		16 px : 1 morceau, 35,7 % du carré, trou de 44 px
//		20 px : 1 morceau, 38,4 %          60 px
//		24 px : 1 morceau, 38,1 %          88 px
//		32 px : 1 morceau, 37,5 %         164 px
//		48 px : 1 morceau, 37,3 %         376 px
//	      128 px : 1 morceau, 37,7 %        2608 px
//
// Un seul morceau à TOUTES les tailles, et une charge qui ne bouge pas : la forme
// ne se délite pas en chemin vers 128 px. Un filet rejoue ces chiffres
// (TestMarqueTientEnUnMorceau), et sa contre-épreuve jouée le fait ROUGIR quand
// l'anneau se bouche — 3 morceaux, 9,5 %, trou de 0 pixel.
//
// L'anneau est un disque évidé ; les têtes sont des disques posés SUR son bord
// extérieur — jamais à côté, la soudure est ce qui fait la tenue — et le bec de
// chaque tête mord l'anneau : c'est l'ouroboros, pas un cercle décoré.
//
// Le motif est un PRÉDICAT et non plus une liste de rectangles : un anneau n'est
// pas fait de rects, et c'est pour ça que la mécanique a suivi — brandIconImage
// appelle marqueMotif, point par point de la grille.
//
// ⚠ Mais la géométrie est AVANT le prédicat, dans marqueDisques, et c'est le point
// qui rend l'unicité vraie au lieu de la promettre. Le « j » vivait en rectangles
// parce que le DESSIN AU SOL (le splash Windows, sys_splash_windows.go) ne savait
// peindre que des rectangles : quatre endroits peignaient la marque, ils avaient
// fallu une liste de formes simples. Le splash ne peint plus des rects, il peint
// des DISQUES ; la marque devient donc une liste de disques, que les deux draweurs
// lisent :
//
//	marqueDisques ──┬─▶ marqueMotif   (prédicat, avec supersampling : l'icône)
//	                └─▶ le splash GDI (disques pleins + un anneau au trait épais)
//
// Une seule géométrie, deux draweurs. C'est ce qui remplace l'ancienne promesse
// « RÉPLIQUE EXACTE du favicon » qui, elle, était fausse (voir l'en-tête).
const (
	marqueCentre    = 6.0  // le centre, en unités de la grille 12
	marqueRayonExt  = 4.70 // l'anneau : bord extérieur
	marqueRayonInt  = 2.70 // l'anneau : bord intérieur (l'épaisseur fait 2.0)
	marqueTeteRayon = 1.02 // une tête : disque posé sur le bord extérieur
	marqueBecRayon  = 0.62 // le bec qui mord l'anneau
)

// marqueTetes : les trois angles des têtes, en degrés. Écartées de 120°, donc
// aucune ne domine : l'hydre n'a pas de tête de devant. Le 0° regarde vers la
// droite, et l'angle croît vers le BAS — le sens de la grille de l'icône.
var marqueTetes = []float64{-60, 60, 180}

// marqueDisques rend la géométrie de la marque, en unités de la grille 12 :
// l'anneau d'abord, puis les têtes et leurs becs.
//
// L'anneau est rendu par son RAYON EXTÉRIEUR et son ÉPAISSEUR, et ce n'est pas une
// commodité : c'est la seule description qui se peigne des deux côtés à la fois.
// En Go on teste les deux bords ; en GDI, Ellipse trace sa bordure À L'INTÉRIEUR
// du rectangle qu'on lui donne — donc un rectangle au rayon extérieur, tracé d'un
// trait de l'épaisseur de l'anneau, couvre exactement de (extérieur − épaisseur) à
// extérieur. GDI ne connaît pas les trous : c'est le seul coup de pinceau possible.
//
// ⚠ J'avais d'abord écrit « rayon moyen », en supposant le trait CENTRÉ sur le
// bord. FAUX en GDI : le trait est INTÉRIEUR, et l'anneau du splash aurait été
// décalé d'une demi-épaisseur par rapport à l'icône — deux draweurs, deux anneaux.
// Corrigé à la relecture de la doc GDI, pas au jugement.
func marqueDisques() (anneauCentre, anneauRayonExt, anneauEpaisseur float64, disques [][3]float64) {
	anneauCentre = marqueCentre
	anneauRayonExt = marqueRayonExt
	anneauEpaisseur = marqueRayonExt - marqueRayonInt
	for _, deg := range marqueTetes {
		r := deg * math.Pi / 180
		cosr, sinr := math.Cos(r), math.Sin(r)
		cx, cy := marqueCentre+marqueRayonExt*cosr, marqueCentre+marqueRayonExt*sinr
		// la tête, posée sur le bord : SOUDÉE, donc le motif tient en un morceau
		disques = append(disques, [3]float64{cx, cy, marqueTeteRayon})
		// le bec : décalé le long de l'anneau, il mord la queue. C'est le détail qui
		// dit l'ouroboros ; à 16 px il se fond dans la tête, et c'est voulu.
		bx := cx - marqueRayonExt*0.22*sinr
		by := cy + marqueRayonExt*0.22*cosr
		disques = append(disques, [3]float64{bx, by, marqueBecRayon})
	}
	return
}

// marqueMotif répond true si le point (gx, gy) de la grille 12x12 appartient à la
// marque. Il lit la géométrie de marqueDisques : il ne peut donc pas en diverger.
func marqueMotif(gx, gy float64) bool {
	c, rExt, e, disques := marqueDisques()
	rInt := rExt - e
	// l'anneau : la zone entre les deux bords
	if d2 := (gx-c)*(gx-c) + (gy-c)*(gy-c); d2 <= rExt*rExt && d2 >= rInt*rInt {
		return true
	}
	for _, d := range disques {
		if hx, hy := gx-d[0], gy-d[1]; hx*hx+hy*hy <= d[2]*d[2] {
			return true
		}
	}
	return false
}

// brandIconImage dessine la marque à la taille n. bg peint le carré arrondi, fg
// le motif (marqueMotif). Les deux peuvent être transparents : c'est ce qui
// produit l'icône « template » de macOS.
func brandIconImage(n int, bg, fg color.RGBA) *image.RGBA {
	const r = 2.0 // rayon des coins, en unités de la grille 12
	outside := func(gx, gy float64) bool {
		corner := func(cx, cy float64) bool {
			dx, dy := gx-cx, gy-cy
			return dx*dx+dy*dy > r*r
		}
		switch {
		case gx < r && gy < r:
			return corner(r, r)
		case gx > 12-r && gy < r:
			return corner(12-r, r)
		case gx < r && gy > 12-r:
			return corner(r, 12-r)
		case gx > 12-r && gy > 12-r:
			return corner(12-r, 12-r)
		}
		return false
	}
	// Chaque pixel est la moyenne de ss×ss échantillons : les coins arrondis et les
	// bords COURBES du motif sont lissés au lieu d'être en escalier. Sans ça, toute
	// taille qui n'est pas un multiple de 12 (20, 32, 40 px : celles de la barre
	// des tâches Windows) donnait des blocs inégaux, puis une icône floue une
	// fois mise à l'échelle par le système.
	//
	// ⚠ Ce supersampling était DÉJÀ là, et c'est tant mieux : il avait été écrit
	// pour les coins arrondis, il fait le travail pour un anneau. Je craignais que
	// tracer un cercle sur une grille 12x12 sans anti-crénelage ne jagote ; la
	// mécanique le faisait déjà, je ne l'avais pas lue. Un anneau se trace donc ici
	// sans rien ajouter — le prédicat du motif suffit.
	const ss = 4
	img := image.NewRGBA(image.Rect(0, 0, n, n))
	scale := float64(n) / 12
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			var r, g, b, a float64 // couleurs prémultipliées par l'alpha
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					gx := (float64(x) + (float64(sx)+0.5)/ss) / scale
					gy := (float64(y) + (float64(sy)+0.5)/ss) / scale
					if outside(gx, gy) {
						continue
					}
					c := bg
					if marqueMotif(gx, gy) {
						c = fg
					}
					ca := float64(c.A) / 255
					r += float64(c.R) * ca
					g += float64(c.G) * ca
					b += float64(c.B) * ca
					a += ca
				}
			}
			const total = ss * ss
			// image.RGBA attend des couleurs prémultipliées.
			img.SetRGBA(x, y, color.RGBA{uint8(r/total + 0.5), uint8(g/total + 0.5), uint8(b/total + 0.5), uint8(a*255/total + 0.5)})
		}
	}
	return img
}

// brandIconSizes : tailles servies par /icon-<n>.png (favicons et manifeste).
var brandIconSizes = map[int]bool{16: true, 32: true, 48: true, 64: true, 96: true, 128: true, 192: true, 256: true, 512: true}

func encodePNG(img *image.RGBA) []byte {
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

// BrandIconPNG rend l'icône de marque (le carré noir, l'hydre-ouroboros blanc) en
// PNG de n pixels. Exporté pour le générateur d'icône du .exe (tools/gen-icon).
func BrandIconPNG(n int) []byte { return encodePNG(brandIconImage(n, brandBlack, brandWhite)) }

// brandTemplatePNG rend la variante « template » attendue par macOS : seule la
// couche alpha compte, le système colore la forme selon le thème de la barre de
// menus. bg = le carré OPAQUE, fg = le motif DÉCOUPÉ (transparent). Un filet le
// vérifie (TestTrouResteTransparentDansLaVarianteMacOS).
//
// ⚠ Le pochoir, et non l'inverse — relu avant d'écrire le filet, qui avait d'abord
// l'attente à l'envers. Une template image est un calque : macOS remplit ce qui est
// OPAQUE avec sa propre couleur. Donc :
//
//	· le MOTIF est transparent : c'est lui qui laisse voir la barre de menus ;
//	· le TROU de l'anneau est OPAQUE, comme le reste du carré. S'il était
//	  transparent, macOS creuserait le trou et colorierait l'anneau : l'anneau
//	  disparaîtrait, remplacé par son propre négatif.
//
// Sans ce découpage, une icône entièrement noire disparaît sur une barre sombre.
//
// Utilisée par sys_tray_darwin.go — un fichier que seule une compilation avec
// CGO voit. Les analyseurs lancés sans CGO la croient morte : elle avait été
// supprimée à ce titre, ce qui a cassé la compilation macOS en CI.
//
//lint:ignore U1000 utilisée par sys_tray_darwin.go, invisible sans CGO/macOS
func brandTemplatePNG(n int) []byte {
	return encodePNG(brandIconImage(n, brandBlack, brandClear))
}

// BrandICO emballe une ou plusieurs tailles PNG dans un conteneur .ico. Windows
// accepte le PNG comme image d'une entrée .ico (alpha conservé pour les coins
// arrondis). Plusieurs tailles = un rendu net partout, de la barre des tâches
// (16 px) à la grande tuile de l'explorateur (256 px).
func BrandICO(sizes ...int) []byte {
	if len(sizes) == 0 {
		sizes = []int{trayIconSize}
	}
	imgs := make([][]byte, 0, len(sizes))
	for _, n := range sizes {
		imgs = append(imgs, BrandIconPNG(n))
	}
	var ico bytes.Buffer
	binary.Write(&ico, binary.LittleEndian, uint16(0))         // réservé
	binary.Write(&ico, binary.LittleEndian, uint16(1))         // type = icône
	binary.Write(&ico, binary.LittleEndian, uint16(len(imgs))) // nombre d'images
	offset := uint32(6 + 16*len(imgs))                         // fin du répertoire
	for i, p := range imgs {
		n := sizes[i]
		// 256 px se code 0 dans un .ico (le champ ne fait qu'un octet).
		ico.WriteByte(byte(n % 256))
		ico.WriteByte(byte(n % 256))
		ico.WriteByte(0)                                        // couleurs de la palette (0 = truecolor)
		ico.WriteByte(0)                                        // réservé
		binary.Write(&ico, binary.LittleEndian, uint16(1))      // plans
		binary.Write(&ico, binary.LittleEndian, uint16(32))     // bits/pixel
		binary.Write(&ico, binary.LittleEndian, uint32(len(p))) // taille données
		binary.Write(&ico, binary.LittleEndian, offset)         // offset données
		offset += uint32(len(p))
	}
	for _, p := range imgs {
		ico.Write(p)
	}
	return ico.Bytes()
}
