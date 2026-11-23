package aidros

// La carte Pont Aidros ↔ Aidros et le nom du projet par défaut, traduits : son
// filet.
//
// Le défaut, vu à l'œil sur une capture et non dans le code : le 23/11, la page
// servie en ANGLAIS gardait « Pont Aidros ↔ Aidros », son badge « à l'écoute »
// et le sélecteur de projet « Générale ». Trois trous, nommés dans mes notes de
// livraison et laissés là. La cause n'était pas une clé mal traduite : ces
// textes n'avaient AUCUNE clé. applyI18n() ne parcourt que les éléments marqués
// data-i18n, et la carte entière — le gabarit comme son JS — était écrite en
// français en dur.
//
// ⚠ Ce que ce filet promet, et ce qu'il ne promet pas. Il garantit la parité des
// clés bridge.* entre fr et en, qu'aucune clé invoquée ne manque au
// dictionnaire, qu'aucune clé écrite ne reste morte, qu'aucun texte VISIBLE de
// la carte du Pont ne s'affiche sans clé, et qu'aucune chaîne française ne
// survit en dur dans son JS. Il ne promet PAS que toute l'application est
// traduite : la garde du texte nu est bornée à la carte du Pont, là où le défaut
// a été MESURÉ. L'étendre au gabarit entier se paierait en faux positifs (le
// français de secours est écrit dans le gabarit, c'est le repli prévu du
// moteur) : l'élargir est une décision, pas un réglage.

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

const (
	fichierI18N    = "ui/src/js/00a-i18n-data.js"
	fichierPont    = "ui/src/js/13b-bridge.js"
	fichierGabarit = "ui/src/index.tmpl.html"
)

// matiereI18N relit TOUT le JS de l'interface PLUS le gabarit. Indispensable,
// pour deux raisons mesurées :
//  1. le JS est éclaté en dix-huit fichiers, et projLabel / projetLabel vivent
//     dans 18-projects.js et 08-chat-render.js, pas dans celui du Pont ;
//  2. les clés posées dans le gabarit le sont en ATTRIBUT HTML (data-i18n=),
//     donc dans un fichier .html, pas .js.
//
// Un filet qui ne lirait que 13b-bridge.js déclarerait « morte » une clé invoquée
// ailleurs — le piège déjà écrit dans ui_reserve_hauteur_test.go (« mon premier
// jet ne lisait que 06-settings.js et criait loadMCP n'existe pas »).
//
// ⚠ sourceJS existe DÉJÀ dans ui_reserve_hauteur_test.go, même paquet, avec le
// même dessein. Mon premier jet redéclarait le même nom : vet l'a refusé
// (« sourceJS redeclared in this block »), et il a eu raison. Deux filets, deux
// matières : la sienne est du JS seul, la mienne ajoute le gabarit.
func matiereI18N(t *testing.T) string {
	t.Helper()
	var bits []string
	for _, dossier := range []string{"ui/src", "ui/src/js"} {
		entrées, err := os.ReadDir(dossier)
		if err != nil {
			t.Fatalf("lire %s : %v", dossier, err)
		}
		for _, e := range entrées {
			if e.IsDir() {
				continue
			}
			n := e.Name()
			if !strings.HasSuffix(n, ".js") && !strings.HasSuffix(n, ".html") {
				continue
			}
			b, err := os.ReadFile(filepath.Join(dossier, n))
			if err != nil {
				t.Fatalf("relire %s/%s : %v", dossier, n, err)
			}
			bits = append(bits, "\n//@@ "+n+"\n"+string(b))
		}
	}
	if len(bits) == 0 {
		t.Fatal("aucun fichier d'interface trouvé : le filet ne garde rien")
	}
	return strings.Join(bits, "\n")
}

// clesI18N relit le dictionnaire et rend, par langue, les clés qui s'y trouvent.
// Le parseur suit les accolades : le fichier porte deux blocs, fr: { ... } puis
// en: { ... }, et des commentaires entre les lignes de clés.
func clesI18N(t *testing.T) map[string]map[string]bool {
	t.Helper()
	b, err := os.ReadFile(fichierI18N)
	if err != nil {
		t.Fatalf("relire %s : %v", fichierI18N, err)
	}
	lignes := strings.Split(string(b), "\n")
	uneCle := regexp.MustCompile(`^\s*"((?:[^"\\]|\\.)*)"\s*:`)
	debutBloc := regexp.MustCompile(`^(fr|en|it|es|ru|de):\s*\{`)
	out := map[string]map[string]bool{}
	for i, l := range lignes {
		m := debutBloc.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		profondeur := 0
		cles := map[string]bool{}
		for k := i; k < len(lignes); k++ {
			for _, c := range lignes[k] {
				if c == '{' {
					profondeur++
				} else if c == '}' {
					profondeur--
				}
			}
			if c := uneCle.FindStringSubmatch(lignes[k]); c != nil {
				cles[c[1]] = true
			}
			if profondeur <= 0 && k > i {
				break
			}
		}
		if len(cles) == 0 {
			t.Fatalf("bloc %s de %s : aucune clé lue, le parseur ne suit plus le fichier", m[1], fichierI18N)
		}
		out[m[1]] = cles
	}
	if len(out["fr"]) == 0 || len(out["en"]) == 0 {
		t.Fatalf("%s : bloc fr ou en introuvable (%d langues lues)", fichierI18N, len(out))
	}
	return out
}

// clesGardees : les clés surveillées par ce filet. bridge.* pour la carte du
// Pont, plus le nom du projet par défaut, qui portait le même défaut — un mot
// figé en français dans le registre Go, affiché tel quel.
func clesGardees(cles map[string]bool) []string {
	var out []string
	for k := range cles {
		if strings.HasPrefix(k, "bridge.") || k == "projects.default_name" {
			out = append(out, k)
		}
	}
	return out
}

// diffEnsembles rend ce que gauche a et droite n'a pas. Comparer des ENSEMBLES
// et non des comptes : deux jeux de taille égale peuvent différer, et une clé
// française sans son pendant anglais ressort en français sur un écran anglais —
// exactement le défaut d'origine.
func diffEnsembles(gauche, droite []string) []string {
	jeu := map[string]bool{}
	for _, k := range droite {
		jeu[k] = true
	}
	var out []string
	for _, k := range gauche {
		if !jeu[k] {
			out = append(out, k)
		}
	}
	return out
}

func TestI18nPontPariteFrancaisAnglais(t *testing.T) {
	cles := clesI18N(t)
	fr := clesGardees(cles["fr"])
	en := clesGardees(cles["en"])
	if len(fr) < 40 {
		t.Fatalf("seulement %d clés surveillées en français : le dictionnaire a-t-il perdu un bloc ?", len(fr))
	}
	if manque := diffEnsembles(fr, en); len(manque) > 0 {
		t.Errorf("%d clé(s) surveillée(s) en français sans pendant anglais — elle(s) resteraient française(s) sur un écran anglais : %v",
			len(manque), manque)
	}
	if manque := diffEnsembles(en, fr); len(manque) > 0 {
		t.Errorf("%d clé(s) surveillée(s) en anglais sans pendant français : %v", len(manque), manque)
	}
	t.Logf("parité ok : %d clés surveillées, dans les deux langues", len(fr))
}

// TestI18nPontClesInvoqueesExistent : toute clé que le code APPELLE doit exister
// dans les deux langues. t() replie sur le français si une clé manque à
// l'anglais, et sur la CLÉ ELLE-MÊME si elle manque partout : un écran anglais
// peut donc afficher « bridge.badge_off » en toutes lettres sans que rien le
// signale à l'écran.
func TestI18nPontClesInvoqueesExistent(t *testing.T) {
	cles := clesI18N(t)
	matiere := matiereI18N(t) // JS éclaté + gabarit, voir l'en-tête de matiereI18N
	invoquees := map[string]bool{}
	appels := regexp.MustCompile(`tp?\(\s*'(bridge\.[a-z0-9_.]+|projects\.default_name)'`)
	marques := regexp.MustCompile(`data-i18n="(bridge\.[a-z0-9_.]+|projects\.default_name)"`)
	for _, m := range appels.FindAllStringSubmatch(matiere, -1) {
		invoquees[m[1]] = true
	}
	for _, m := range marques.FindAllStringSubmatch(matiere, -1) {
		invoquees[m[1]] = true
	}
	if len(invoquees) < 40 {
		t.Fatalf("seulement %d clé(s) surveillée(s) invoquée(s) par le code : la traduction a-t-elle régressé ?", len(invoquees))
	}
	var absentes []string
	for k := range invoquees {
		if !cles["fr"][k] || !cles["en"][k] {
			absentes = append(absentes, k)
		}
	}
	if len(absentes) > 0 {
		t.Errorf("%d clé(s) invoquée(s) par le code et absente(s) du dictionnaire — l'écran affichera la clé en toutes lettres : %v",
			len(absentes), absentes)
	}
}

// TestI18nPontAucuneCleMorte : l'inverse. Une clé écrite et jamais lue est une
// clé qu'on croit gardée ; elle ne coûte rien à l'écran et tout à la relecture.
func TestI18nPontAucuneCleMorte(t *testing.T) {
	cles := clesI18N(t)
	matiere := matiereI18N(t)
	var mortes []string
	for _, k := range clesGardees(cles["fr"]) {
		if !strings.Contains(matiere, "'"+k+"'") && !strings.Contains(matiere, `"`+k+`"`) {
			mortes = append(mortes, k)
		}
	}
	if len(mortes) > 0 {
		t.Errorf("%d clé(s) surveillée(s) écrite(s) dans le dictionnaire et invoquée(s) nulle part : %v", len(mortes), mortes)
	}
}

// TestI18nPontGabaritSansTexteNu : dans la carte du Pont du gabarit, tout texte
// VISIBLE doit être porté par data-i18n. C'est la règle qu'applyI18n() impose
// d'elle-même : un texte sans clé ne se traduit pas, il reste ce qui est écrit.
//
// ⚠ Le français de secours reste ÉCRIT dans le gabarit, à côté de sa clé : c'est
// le repli prévu du moteur (voir l'en-tête de 00b-i18n.js). Ce filet n'interdit
// donc pas le français, il interdit le français SANS CLÉ.
func TestI18nPontGabaritSansTexteNu(t *testing.T) {
	b, err := os.ReadFile(fichierGabarit)
	if err != nil {
		t.Fatalf("relire %s : %v", fichierGabarit, err)
	}
	// Les commentaires sont français par contrat : ils ne s'affichent pas. Ils
	// sont retirés EN PRESERVANT la numérotation (un retour à la ligne remplace
	// chaque saut de ligne du commentaire), sinon les lignes citées en échec
	// mentiraient sur l'endroit où regarder.
	sansCommentaires := regexp.MustCompile(`(?s)<!--.*?-->`).ReplaceAllStringFunc(string(b), func(s string) string {
		return strings.Repeat("\n", strings.Count(s, "\n"))
	})
	lignes := strings.Split(sansCommentaires, "\n")
	debut := -1
	for i, l := range lignes {
		if strings.Contains(l, `id="bridge-details"`) {
			debut = i
			break
		}
	}
	if debut < 0 {
		t.Fatalf("%s : la carte bridge-details est introuvable — le filet ne garde plus rien", fichierGabarit)
	}
	// Trois lettres : assez pour un mot, assez pour ne pas chasser « 32 » ni une
	// flèche. Seuls les ÉLÉMENTS portent data-i18n ; un mot dans un ATTRIBUT
	// (title=, placeholder=) est hors de ce contrôle, il est traduit par
	// data-i18n-attr, que le gabarit utilise déjà — et que je vérifie à part.
	texte := regexp.MustCompile(`>([^<>]*\p{L}\p{L}\p{L}[^<>]*)<`)
	var nu []string
	vus := 0
	for i := debut; i < len(lignes); i++ {
		l := lignes[i]
		if i > debut && strings.Contains(l, "</details>") {
			break
		}
		for _, m := range texte.FindAllStringSubmatch(l, -1) {
			if strings.TrimSpace(m[1]) == "" {
				continue
			}
			vus++
			if !strings.Contains(l, "data-i18n") {
				nu = append(nu, "L"+strconv.Itoa(i+1)+" « "+strings.TrimSpace(m[1])+" »")
			}
		}
	}
	if vus < 12 {
		t.Fatalf("seulement %d fragment(s) de texte trouvé(s) dans la carte du Pont : le parseur ne suit plus le gabarit", vus)
	}
	if len(nu) > 0 {
		t.Errorf("%d texte(s) visible(s) de la carte du Pont sans data-i18n — ils ne se traduiront jamais :\n  %s",
			len(nu), strings.Join(nu, "\n  "))
	}
	t.Logf("%d fragments de texte contrôlés dans la carte du Pont", vus)
}

// accents : le français en porte, les identifiants JS n'en portent pas. C'est le
// critère, et il est plus sûr qu'une liste de mots : ma première liste accusait
// action: 'presets' — un identifiant d'API, pas du texte.
//
// ⚠ Ce qu'il manque : un mot français SANS accent et sans apostrophe
// typographique. La carte en portait le 23/11 : « Ajouter », « Oublier »,
// « Retirer », « Activer », « Moteurs ». Ils sont tous traduits, et la liste
// fermée motsSansAccent les surveille un à un. C'est une liste, elle n'est pas
// la langue : c'est le prix, nommé, d'une garde lexicalisée.
//
// ⚠ LA LISTE NE GARDE QUE DES MOTS QUI NE PEUVENT PAS ÊTRE DES IDENTIFIANTS JS.
// Ma première liste portait « moteur », « machine », « voisin », « adresse »,
// « erreur » : le filet est tombé sur `const moteur = pre.moteur ? …` et l'a
// accusé d'être du français. Ces cinq mots servent d'identifiants dans ce
// fichier ; les retirer est le correctif, pas un arrosage, car l'accentUE les
// couvre déjà dès qu'ils sont accentués (« épinglée », « refusé », « modèle »).
//
// CE QUE ÇA COÛTE, nommé : un mot français sans accent et sans majuscule
// (« prêt », « ici ») écrit en dur dans ce fichier passerait. La liste fermée
// ci-dessous ne prétend pas connaître la langue ; elle connaît les mots d'écran
// de CETTE carte, mesurés le 23/11, et les identifiants n'en font pas partie.
var (
	accentUE       = regexp.MustCompile(`[àâäçéèêëîïôùûüœÀÂÄÇÉÈÊËÎÏÔÙÛÜŒ]`)
	motFrancais    = regexp.MustCompile(`'[^']*\p{L}[^']*'`)
	litterelDouble = regexp.MustCompile(`"[^"]*\p{L}[^"]*"`)
	motsSansAccent = []string{
		// Les mots d'écran de la carte du Pont, sans accent, tels qu'ils
		// s'affichaient en dur le 23/11. Ajoutés un à un, pas une liste au jugé.
		"Ajouter", "Oublier", "Retirer", "Activer", "Moteurs",
		"Recevoir", "Parler", "cerveau", "coller",
	}
)

// TestI18nPontJsSansChaineFrancaise : dans 13b-bridge.js, tout texte destiné à
// l'écran passe par t() ou tp(). Le défaut d'origine était exactement ça : des
// chaînes françaises littérales collées dans textContent et dans innerHTML.
func TestI18nPontJsSansChaineFrancaise(t *testing.T) {
	b, err := os.ReadFile(fichierPont)
	if err != nil {
		t.Fatalf("relire %s : %v", fichierPont, err)
	}
	var fautives []string
	lignes := strings.Split(string(b), "\n")
	for i, brut := range lignes {
		l := strings.TrimSpace(brut)
		if strings.HasPrefix(l, "//") {
			continue // un commentaire est français par contrat
		}
		if teneur := chaineAvecAccent(l); teneur != "" {
			fautives = append(fautives, "L"+strconv.Itoa(i+1)+" (accentuée) "+tronc(teneur))
			continue
		}
		for _, mot := range motsSansAccent {
			if regexp.MustCompile(`'[^']*\b` + mot + `\b[^']*'`).MatchString(l) {
				fautives = append(fautives, "L"+strconv.Itoa(i+1)+" (« "+mot+" ») "+tronc(l))
				break
			}
		}
	}
	if len(fautives) > 0 {
		t.Errorf("%d ligne(s) de %s portent un texte français en dur — il ne se traduira pas :\n  %s",
			len(fautives), filepath.Base(fichierPont), strings.Join(fautives, "\n  "))
	}
}

// chaineAvecAccent rend le premier littéral de la ligne qui porte un caractère
// accentué, ou une apostrophe typographique (le français de la carte en était
// plein : « l'autre Aidros »). Les clés t('bridge.titre') sont épargnées par
// construction : une clé est en ASCII.
func chaineAvecAccent(l string) string {
	for _, m := range motFrancais.FindAllString(l, -1) {
		if accentUE.MatchString(m) || strings.Contains(m, "’") {
			return m
		}
	}
	for _, m := range litterelDouble.FindAllString(l, -1) {
		if accentUE.MatchString(m) || strings.Contains(m, "’") {
			return m
		}
	}
	return ""
}

func tronc(s string) string {
	if len(s) > 120 {
		return s[:120] + "…"
	}
	return s
}

// TestI18nPontNomProjetTraduit : le troisième trou mesuré. Le serveur fige
// defaultProjectName = « Générale » et l'affichait tel quel. Ce qui doit rester
// vrai : le NOM STOCKÉ ne bouge pas (c'est lui qui nomme memory/<slug>/), et les
// routes qui l'exposent disent bien si le projet porte encore ce nom figé — sans
// ce drapeau, on écraserait un nom que l'humain a écrit.
func TestI18nPontNomProjetTraduit(t *testing.T) {
	if defaultProjectName != "Générale" {
		t.Errorf("defaultProjectName = %q : le nom STOCKÉ doit rester le nom français figé, c'est lui qui nomme memory/<slug>/ ; ce n'est pas lui qu'on traduit", defaultProjectName)
	}
	if b, err := os.ReadFile("web_projects.go"); err != nil {
		t.Fatalf("relire web_projects.go : %v", err)
	} else if s := string(b); !strings.Contains(s, "name_default") {
		t.Error("web_projects.go ne rend plus name_default : l'interface ne peut plus savoir si le projet par défaut a été renommé, et traduirait un nom que l'humain a écrit")
	} else if !strings.Contains(s, "p.Name == defaultProjectName") {
		t.Error("web_projects.go ne compare plus le nom au nom figé : name_default ne veut plus dire « jamais renommé »")
	}
	if b, err := os.ReadFile("web_chat.go"); err != nil {
		t.Fatalf("relire web_chat.go : %v", err)
	} else if !strings.Contains(string(b), "default_name_default") {
		t.Error("web_chat.go ne rend plus default_name_default : l'historique afficherait « Générale » sur un écran anglais")
	}
}
