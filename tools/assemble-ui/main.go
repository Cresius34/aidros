// assemble-ui reconstruit internal/aidros/ui/index.html à partir des sources
// internal/aidros/ui/src/ (remplace l'ancien ui/assemble.ps1 — multiplateforme).
//
// index.html est un fichier GÉNÉRÉ mais committé : go:embed (web_server.go) et
// ajean-app/build-server-ui.ps1 le lisent tel quel. Pour modifier l'UI : éditer
// ui/src/ (index.tmpl.html, styles.css, js/NN-*.js concaténés dans l'ordre
// alphabétique, tout en scope global) puis relancer :
//
//	go generate ./internal/aidros        (ou : go run ./tools/assemble-ui)
//
// Les marqueurs @@CSS@@ / @@JS@@ du template sont remplacés avec LEUR fin de
// ligne (LF ou CRLF) ; chaque source apporte ses propres fins de ligne.
//
// ⭐ @@MARQUE@@ et @@MARQUE_INLINE@@ viennent du CODE GO, pas d'un fichier : la
// marque (l'hydre-ouroboros) se dessine une seule fois, dans internal/aidros
// (sys_brand_icon.go), et ce générateur la pose là où l'interface en a besoin —
// le favicon, le voile de chargement, la ligne d'activité, le manifeste PWA,
// l'icône des notifications push. Mesuré le 22/11 au soir, ces cinq endroits
// portaient QUATRE copies écrites à la main du « J » d'avant, et l'humain le
// voyait encore à l'écran. Un filet qui surveille une copie ne l'empêche pas
// d'être périmée, il le dit APRÈS. Dessiner une fois règle les cinq à la fois.
package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/nathaninline/ajean/internal/aidros"
)

func main() {
	uiDir := "internal/aidros/ui"
	if len(os.Args) > 1 {
		uiDir = os.Args[1]
	}
	if err := run(uiDir); err != nil {
		fmt.Fprintln(os.Stderr, "assemble-ui:", err)
		os.Exit(1)
	}
}

func run(uiDir string) error {
	srcDir := filepath.Join(uiDir, "src")
	tmpl, err := os.ReadFile(filepath.Join(srcDir, "index.tmpl.html"))
	if err != nil {
		return err
	}
	css, err := os.ReadFile(filepath.Join(srcDir, "styles.css"))
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(filepath.Join(srcDir, "js"))
	if err != nil {
		return err
	}
	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".js") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	var js []byte
	for _, n := range names {
		b, err := os.ReadFile(filepath.Join(srcDir, "js", n))
		if err != nil {
			return err
		}
		js = append(js, b...)
	}

	// Commentaires de développement retirés de la page publiée (voir strip.go). Le
	// JS nettoyé est vérifié équivalent à l'original avant d'être écrit.
	jsOut, err := stripJSComments(string(js))
	if err != nil {
		return fmt.Errorf("retrait des commentaires JS : %w", err)
	}
	if err := verifySameJS(string(js), jsOut); err != nil {
		return err
	}
	cssOut, err := stripCSSComments(string(css))
	if err != nil {
		return err
	}
	html := stripHTMLComments(string(tmpl))
	for marker, content := range map[string]string{"@@CSS@@": cssOut, "@@JS@@": jsOut} {
		re := regexp.MustCompile(regexp.QuoteMeta(marker) + "\r?\n")
		if !re.MatchString(html) {
			return fmt.Errorf("marqueur %s introuvable dans index.tmpl.html", marker)
		}
		html = re.ReplaceAllStringFunc(html, func(string) string { return content })
	}

	// La marque, dessinée par le code Go du paquet (voir l'en-tête de ce fichier).
	// Ces deux marqueurs sont EN LIGNE — dans un href, dans une chaîne JS — et non
	// seuls sur leur ligne comme le CSS et le JS : d'où un remplacement littéral,
	// qui ne se pose pas la question d'un éventuel `$1`.
	marque := aidros.BrandMarqueSVG()
	marqueNu := aidros.BrandMarqueSVGInline()
	// apple-touch-icon : iOS n'accepte pas le SVG pour l'icône d'accueil, il lui
	// faut un PNG. BrandIconPNG le dessine — le SIXIÈME dessin de la marque était
	// un PNG en base64 relu DIFFÉRENT de cmd/aidros/icon.png du dépôt (mesuré
	// 22/11, l'en-tête de sys_brand_icon.go le racontait). Il sort donc du même
	// code, à 180 px comme l'exige iOS.
	marquePNG180 := base64.StdEncoding.EncodeToString(aidros.BrandIconPNG(180))
	for _, m := range []struct{ marqueur, valeur string }{
		{"@@MARQUE@@", marque},
		{"@@MARQUE_INLINE@@", marqueNu},
		{"@@MARQUE_PNG_180@@", marquePNG180},
	} {
		if !strings.Contains(html, m.marqueur) {
			return fmt.Errorf("marqueur %s introuvable dans index.tmpl.html : la marque ne serait plus dessinée là où l'interface l'attend", m.marqueur)
		}
		html = strings.ReplaceAll(html, m.marqueur, m.valeur)
	}

	// Le manifeste PWA et le service worker portent la marque eux aussi — l'icône
	// d'installation, l'icône des notifications push. Comme le reste des sources ils
	// vivent sous ui/src/ avec @@MARQUE@@ ; leurs copies sous ui/ sont le produit,
	// go:embed les embarque (web_server.go:22). Leurs commentaires sont GARDÉS :
	// ce sont des fichiers qu'on lit à la main, pas la page publiée.
	for _, nom := range []string{"manifest.webmanifest", "sw.js"} {
		brut, err := os.ReadFile(filepath.Join(srcDir, nom))
		if err != nil {
			return err
		}
		if n := strings.Count(string(brut), "@@MARQUE@@"); n == 0 {
			return fmt.Errorf("%s : aucun @@MARQUE@@ — la marque a disparu de cette source, et l'écrire sans elle produirait un fichier qui la nomme sans la dessiner", nom)
		}
		if err := os.WriteFile(filepath.Join(uiDir, nom),
			[]byte(strings.ReplaceAll(string(brut), "@@MARQUE@@", marque)), 0o644); err != nil {
			return err
		}
	}

	dst := filepath.Join(uiDir, "index.html")
	if err := os.WriteFile(dst, []byte(html), 0o644); err != nil {
		return err
	}
	fmt.Printf("OK -> %s (%d octets)\n", dst, len(html))
	return nil
}
