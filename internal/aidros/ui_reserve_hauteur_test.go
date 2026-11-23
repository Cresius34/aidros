package aidros

// La réserve de hauteur des six blocs peuplés par le réseau : son filet.
//
// Le défaut, mesuré sur le poste de Julien le 23/11 : la réserve ne tombait
// qu'après les DIX-NEUF chargeurs. Or le plus lent, /api/moe à 2,396 s (les
// dix-huit autres à 0,079 s ou moins), ne peint aucun des six : il les retenait
// tous. Six panneaux restaient bloqués ~2,4 s pour un bloc qui n'a rien à voir
// avec eux. Le gain, joué sous Node avec les durées mesurées : 2402 ms →
// 29-75 ms par bloc. Ce filet, lui, ne mesure pas le gain — il empêche le lien
// de se défaire.
//
// ⚠ Le JS est ÉCLATÉ en dix-huit fichiers : loadCfg et loadTelemetry sont dans
// 05-status.js, loadMCP dans 12-mcp.js. Mon premier jet ne lisait que
// 06-settings.js et criait « loadMCP n'existe pas » — un rouge pour une raison
// sans rapport. Le lien id → chargeur se vérifie donc dans TOUT le JS, comme
// url_ouverture_test.go qui parcourt ui/src et ui/src/js avant moi.
//
// ⚠ Un id sans propriétaire retombe sur releaseHeights : rien n'est cassé à
// l'écran, le bloc est juste encore otage des dix-huit. C'est une régression
// muette — la seule façon de la voir est de compter.

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const fichierReserve = "ui/src/js/06-settings.js"

// sourceJS relit TOUS les fichiers JS de l'interface, séparés par une ligne.
// C'est la matière du filet : un chargeur peut être n'importe où.
func sourceJS(t *testing.T) string {
	t.Helper()
	var bits []string
	for _, dossier := range []string{"ui/src", "ui/src/js"} {
		entrées, err := os.ReadDir(dossier)
		if err != nil {
			t.Fatalf("lire %s : %v", dossier, err)
		}
		for _, e := range entrées {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".js") {
				continue
			}
			b, err := os.ReadFile(filepath.Join(dossier, e.Name()))
			if err != nil {
				t.Fatalf("relire %s/%s : %v", dossier, e.Name(), err)
			}
			bits = append(bits, "\n//@@ "+e.Name()+"\n"+string(b))
		}
	}
	if len(bits) == 0 {
		t.Fatal("aucun fichier JS trouvé : le filet ne garde rien")
	}
	return strings.Join(bits, "\n")
}

// sourceReserve relit le seul fichier qui porte la réserve.
func sourceReserve(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(fichierReserve)
	if err != nil {
		t.Fatalf("relire %s : %v", fichierReserve, err)
	}
	return string(b)
}

// tableauJS relit un tableau JS de chaînes : `const X=['a','b'];`
func tableauJS(t *testing.T, source, constName string) []string {
	t.Helper()
	m := regexp.MustCompile(`(?s)const\s+` + regexp.QuoteMeta(constName) + `\s*=\s*\[(.*?)\]`).FindStringSubmatch(source)
	if m == nil {
		t.Fatalf("%s est introuvable dans %s", constName, fichierReserve)
	}
	var out []string
	for _, s := range regexp.MustCompile(`'([^']*)'|"([^"]*)"`).FindAllStringSubmatch(m[1], -1) {
		out = append(out, premierNonVide(s[1], s[2]))
	}
	return out
}

// objetJS relit un objet JS de chaînes paires : `const X={'a':'b', c:'d'};`
// ⚠ Les clés peuvent être SANS guillemets : mon correctif écrit `cfg:'loadCfg'`.
// Mon premier regex n'acceptait que les clés quotées et perdait cfg, presets et
// vram — trois rouges pour une raison sans rapport avec le correctif.
func objetJS(t *testing.T, source, constName string) map[string]string {
	t.Helper()
	m := regexp.MustCompile(`(?s)const\s+` + regexp.QuoteMeta(constName) + `\s*=\s*\{(.*?)\}`).FindStringSubmatch(source)
	if m == nil {
		t.Fatalf("%s est introuvable dans le JS de la réserve", constName)
	}
	out := map[string]string{}
	for _, s := range regexp.MustCompile(`(?:"([^"]+)"|'([^']+)'|([A-Za-z0-9_$-]+))\s*:\s*(?:"([^"]+)"|'([^']+)')`).FindAllStringSubmatch(m[1], -1) {
		cle := premierNonVide(s[1], premierNonVide(s[2], s[3]))
		valeur := premierNonVide(s[4], s[5])
		if cle != "" && valeur != "" {
			out[cle] = valeur
		}
	}
	return out
}

// corpsDeFonctionJS relit le corps d'une fonction JS, borné à sa fonction :
// chercher un marqueur dans tout le fichier le trouverait ailleurs.
// ⚠ Ne PAS réutiliser corpsDeFonction (mem_embeddings_cache_disque_test.go) :
// il cherche « func », le mot Go. Dans du JS, il ne trouve jamais rien, et un
// filet qui ne trouve rien peut passer pour un filet qui garde bien.
func corpsDeFonctionJS(t *testing.T, fichier, fonction string) string {
	t.Helper()
	b, err := os.ReadFile(fichier)
	if err != nil {
		t.Fatalf("relire %s : %v", fichier, err)
	}
	src := string(b)
	i := strings.Index(src, "function "+fonction+"(")
	if i < 0 {
		i = strings.Index(src, "async function "+fonction+"(")
	}
	if i < 0 {
		t.Fatalf("%s est absente de %s", fonction, fichier)
	}
	o := strings.Index(src[i:], "{")
	if o < 0 {
		t.Fatalf("%s n'a pas d'accolade ouvrante dans %s", fonction, fichier)
	}
	fin := strings.Index(src[i+o:], "\n}")
	if fin < 0 {
		t.Fatalf("%s n'a pas d'accolade fermante en début de ligne dans %s", fonction, fichier)
	}
	return src[i+o : i+o+fin]
}

func premierNonVide(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// corpsDeFonctionDansJS relit le corps d'une fonction JS dans la source
// CONCATÉNÉE (tous les fichiers), en nommant le fichier où elle vit. loadMoe
// n'est ni dans 06-settings.js ni dans 10b-moe.js : un filet qui impose son
// fichier crie « absente » pour une raison sans rapport.
func corpsDeFonctionDansJS(t *testing.T, js, fonction string) string {
	t.Helper()
	for _, morceau := range strings.Split(js, "\n//@@ ") {
		i := strings.Index(morceau, "function "+fonction+"(")
		if i < 0 {
			continue
		}
		o := strings.Index(morceau[i:], "{")
		if o < 0 {
			continue
		}
		fin := strings.Index(morceau[i+o:], "\n}")
		if fin < 0 {
			continue
		}
		return morceau[i+o : i+o+fin]
	}
	t.Fatalf("%s est absente de tout le JS de l'interface", fonction)
	return ""
}

// corpsDeLoadAll relit le corps de loadAll, borné à sa fonction : chercher un
// marqueur dans tout le fichier le trouverait dans reserveHeights.
func corpsDeLoadAll(t *testing.T, source string) string {
	t.Helper()
	i := strings.Index(source, "async function loadAll()")
	if i < 0 {
		t.Fatalf("loadAll est absente de %s", fichierReserve)
	}
	o := strings.Index(source[i:], "{")
	if o < 0 {
		t.Fatalf("loadAll n'a pas d'accolade ouvrante")
	}
	fin := strings.Index(source[i+o:], "\n}")
	if fin < 0 {
		t.Fatalf("loadAll n'a pas d'accolade fermante en début de ligne")
	}
	return source[i+o : i+o+fin]
}

func TestReserveDeHauteurLeveeParSonChargeur(t *testing.T) {
	js := sourceJS(t)
	source := sourceReserve(t)

	// 1. Les six blocs réservés, et le lien id → chargeur qui les peint.
	reserves := tableauJS(t, source, "HRESERVE")
	if len(reserves) != 6 {
		t.Fatalf("%d blocs réservés (%v), attendu 6", len(reserves), reserves)
	}
	proprietaire := objetJS(t, source, "HPROPRIETAIRE")
	for _, id := range reserves {
		// ⚠ Le cœur du filet. Sans propriétaire, `if(!p) continue` fait retomber
		// le bloc sur releaseHeights : il n'est PAS cassé, il est juste encore
		// otage des dix-huit. Rien à l'écran ne le dirait.
		if proprietaire[id] == "" {
			t.Errorf("le bloc réservé « %s » n'a pas de chargeur propriétaire : il attendrait les dix-huit, dont /api/moe à 2,4 s qui ne le peint pas", id)
		}
	}
	for id, nom := range proprietaire {
		if !estDedans(reserves, id) {
			t.Errorf("HPROPRIETAIRE nomme « %s », absent de HRESERVE : un id renommé casse le lien en silence", id)
		}
		// Le propriétaire doit exister QUELQUE PART dans le JS.
		if !strings.Contains(js, "function "+nom+"(") {
			t.Errorf("le bloc « %s » est confié à %s, qu'aucun fichier JS ne définit", id, nom)
		}
	}

	corps := corpsDeLoadAll(t, source)

	// 2. Les dix-huit chargeurs sont tous lancés, et UN SEUL APPEL chacun.
	// ⚠ Le lancement passe par `.map(f=>lance(f.name,f))` : les noms ne sont PAS
	// littéraux dans `lance(...)`. La liste à compter est le tableau lui-même.
	// Un chargeur retiré = un panneau qui ne se remplit plus ; deux = deux
	// appels réseau pour une ligne d'affichage.
	liste := regexp.MustCompile(`(?s)const\s+T\s*=\s*\[(.*?)\]\s*\.\s*map`).FindStringSubmatch(corps)
	if liste == nil {
		t.Fatal("la liste des chargeurs de loadAll est introuvable : le tableau passé à allSettled a changé de forme")
	}
	noms := regexp.MustCompile(`\b(load[A-Za-z0-9_$]+)`).FindAllString(liste[1], -1)
	if len(noms) != 18 {
		t.Errorf("%d chargeurs dans la liste, attendu 18 : %v", len(noms), noms)
	}
	compte := map[string]int{}
	for _, nom := range noms {
		compte[nom]++
	}
	for nom, n := range compte {
		if n > 1 {
			t.Errorf("%s apparaît %d fois dans la liste : deux appels réseau pour une ligne d'affichage", nom, n)
		}
	}
	// Chaque propriétaire est de la partie : un propriétaire hors tableau ne
	// résout jamais sa promesse, et son bloc resterait sous réserve.
	for id, nom := range proprietaire {
		if compte[nom] != 1 {
			t.Errorf("le propriétaire %s du bloc « %s » n'est pas lancé exactement une fois par loadAll (lancé %d fois)", nom, id, compte[nom])
		}
	}
	// Le cache de promesses est ce qui garantit l'appel unique : sans lui,
	// relancer pour attendre équivaudrait à deux requêtes.
	if !strings.Contains(corps, "if(!P[nom]) P[nom]=f()") {
		t.Error("le cache de promesses a disparu : attendre un chargeur redemanderait le réseau")
	}

	// 3. Un échec doit aussi rendre la main. C'est précisément le cas allSettled
	// — accès distant coupé, clé API absente — que le commentaire du fichier
	// invoque depuis le départ. Un .then à une branche le laisserait en réserve.
	if len(regexp.MustCompile(`\.then\(\s*\(\)\s*=>\s*libereBloc\([^)]+\)\s*,\s*\(\)\s*=>\s*libereBloc\(`).FindAllStringSubmatch(corps, -1)) == 0 {
		t.Error("la levée de réserve ne gère pas l'échec : un chargeur en échec laisserait son bloc sous réserve éternelle")
	}
	// 4. Le filet reste : releaseHeights est toujours appelé après tout.
	if !strings.Contains(corps, "releaseHeights()") {
		t.Error("releaseHeights n'est plus appelé dans loadAll : plus aucun filet si un propriétaire manque")
	}
	// 5. libereBloc mesure et écrit pour la prochaine fois : c'est ce qui rend la
	// réserve utile d'une session à l'autre. Retiré, le saut revient.
	corpsLibere := corpsDeFonctionJS(t, fichierReserve, "libereBloc")
	if !strings.Contains(corpsLibere, "ajean-h-") {
		t.Error("libereBloc n'écrit plus la hauteur mesurée : la réserve ne servirait plus à rien la fois d'après")
	}
	if !strings.Contains(corpsLibere, "minHeight=''") {
		t.Error("libereBloc ne rend plus la main au bloc")
	}
	// 6. La réserve se pose toujours au démarrage.
	if !strings.Contains(source, "reserveHeights") || !strings.Contains(source, "DOMContentLoaded") {
		t.Error("reserveHeights n'est plus branché sur DOMContentLoaded : plus de réserve du tout")
	}
}

// Le bloc ne doit pas être libéré avant d'être peuplé : la réserve a ce rôle
// précis (le commentaire du fichier le dit — « la réserve tombe dès que le vrai
// contenu est là »). Appeler libereBloc en ligne, hors promesse, la ferait
// tomber avant toute réponse réseau.
func TestReserveTombeApresLaPeinture(t *testing.T) {
	js := sourceJS(t)
	source := sourceReserve(t)
	corps := corpsDeLoadAll(t, source)

	if strings.Contains(corps, "libereBloc(id);") && !strings.Contains(corps, ".then(") {
		t.Error("libereBloc est appelé en ligne, hors promesse : la réserve tomberait avant la peinture")
	}

	// Chaque propriétaire peint bien son bloc : sinon le lien est un voeu. Les
	// marqueurs sont ceux MESURÉS ce soir (le chargeur, ou le rendu qu'il
	// appelle), dans le fichier où il vit réellement.
	peinture := map[string]string{
		"loadCfg":       "getElementById('cfg')",
		"loadPresets":   "getElementById('presets')",
		"loadInternet":  "getElementById('internet-status')",
		"loadMCP":       "getElementById('mcp-list')",
		"loadTelemetry": "renderVram",
	}
	for nom, marqueur := range peinture {
		if !strings.Contains(js, marqueur) {
			t.Errorf("%s ne peint plus rien de connu (%s introuvable) : le lien id → chargeur ment", nom, marqueur)
		}
	}
	// Et le plus lent ne peint aucun des six : c'est LA cause du défaut, à
	// garder nommée. S'il en peignait un, le diagnostic initial est à reprendre.
	corpsMoe := corpsDeFonctionDansJS(t, js, "loadMoe")
	for _, id := range tableauJS(t, source, "HRESERVE") {
		if strings.Contains(corpsMoe, "getElementById('"+id+"')") {
			t.Errorf("loadMoe peint dans %s : le diagnostic « ne peint aucun des six » est faux, le lien HPROPRIETAIRE doit le nommer", id)
		}
	}
}

func estDedans(liste []string, s string) bool {
	for _, v := range liste {
		if v == s {
			return true
		}
	}
	return false
}
