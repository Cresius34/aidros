package aidros

// Set by the dedicated portable entry point. Keeping this build
// identity in the executable also protects a direct double-click (no launcher
// or inherited AJEAN_HOME is required to keep production isolated).
import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

var BuildFlavor string

// parallelMemoryRevision identifie la génération d'interface servie (champ API
// memory_ui_revision). Il n'apparaît plus comme repère visuel depuis la
// v0.18.4.1 Cresius : le bandeau affiche un intitulé, plus un numéro de revue.
const parallelMemoryRevision = 7

// buildFlavorSuffix est l'identité publique de ce build. Depuis la scission
// actée le 21/11, la base n'est PLUS « reprise telle quelle du tag nathaninline
// fusionné » : elle est la nôtre, et le nom du patch n'entre plus dans le
// millésime (ordre de l'humain du 21/11 : « supprime Cresius du millésime »).
// Version = "1.0" suivi de buildFlavorSuffix rend le millésime affiché : « .0 » a
// rendu « 1.0.0 » (étiquette git v1.0.0+rev0), « .1 » rend « 1.0.1 » (ordre de
// l'humain du 23/11). Le suffixe reste un incrément de correctif en « . », jamais
// un « + » : c'est la forme qu'admet le test de version Windows, et celle que le
// lanceur d'un poste sait lire.
//
// Le « + » n'était pas qu'un goût. Il rendait la release INVISIBLE à l'auto-maj :
// semver.Canonical efface le build-suffixe, donc Compare(v1.0.0+rev0, v1.0) rend 0
// et rien n'était jamais proposé (mesuré avec tools/sonde-semver, la sonde
// versionnée). Un « . » se compare : 1.0.1 > 1.0.0.
//
// Ce qui GARDE encore le nom du patch, et ne le perd pas avec cette décision :
// l'étiquette git (+revN), l'intitulé affiché sous le logo (uiBadgeLabel), et
// le verrou $expectedVersion du lanceur de l'autre poste.
//
// HISTORIQUE de la lignée, sur les bases amont 0.18.x puis 0.20.1 (la
// numérotation d'avant la scission, conservée pour que les étiquettes +rev*
// restent lisibles). Sur la base 0.18.15 : « 0.18.15.5 Cresius » (tag git
// v0.18.15+rev5) : la révision 1 était le patch correctif, la 2
// ajoute le canal Aidros↔Aidros (pont + outil peer), la 3 rend ce canal honnête —
// le modèle prêté sait enfin qu'il est prêté et qu'il n'a aucun outil, et un
// refus du build portable explique POURQUOI il refuse ; la 4 fait rendre au pont
// un véritable tour d'agent de l'autre côté, sous double autorisation ; la 5
// distingue l'AUTEUR de chaque message du canal — un ordre d'un humain qui vise
// un poste lie l'IA de ce poste, un ordre d'une IA n'en lie aucune, et le tour
// lancé pour un appelant qui raccroche est annulé au lieu de tourner pour
// personne.
// Sur la base 0.18.16 : la révision 1 est le patch canal 18161 ; la 2 applique enfin
// la réserve VRAM choisie dans l'application — le champ était décoratif, user_args du
// profil moteur gagnait à chaque démarrage —, rend le seuil de compactage réglable en
// pourcentage de la fenêtre (COMPACT_FRAC), et répare l'outil peer : une action
// perdue rendait la liste des voisins sans un mot d'erreur, ce qui, vu d'en face,
// ressemblait à une réponse du voisin. La 3 monte le moteur Strata de 0.1.48 à 0.1.54
// (paquet 0.2.8). La 4 fait lire au panneau de modèles le CATALOGUE VIVANT du moteur
// au lieu d'une liste recopiée à la main : Ajean proposait 2 familles et 4
// quantifications là où le moteur posé en connaît 8 et 25, avec leurs coûts réels et
// l'avis calibré sur cette machine. Le nommage est ouvert aux familles que setup.py
// sait installer (garde-fou par forme, plus par liste fermée), un couple famille/taille
// inconnu ne rend plus un coût à zéro — qui faisait annoncer « ça tient » à tort —, et
// l'installation accepte un dossier de poids déjà descendus à la main (--gguf-dir).
// Sur la base 0.18.17 : la révision 1 est la fusion de l'amont lui-même — l'erreur d'envoi
// puis le redémarrage après un compactage en plein tour (#181, la ligne même de notre seuil
// de compactage), l'effort de réflexion envoyé en reasoning_effort natif aux API externes
// qui le rejetaient (#178), la VRAM inversée sur portable où la puce Radeon intégrée passait
// pour une carte dédiée (#182, et c'est bien ce chemin qui nous sert : amd-smi est refusé sur
// Windows, le repli Vulkan fait foi), le mot d'activité des tours terminés qui masquait durée
// et débit (#173), plus la voix de Jean et le classement des fichiers par le sens.
// Sur la base 0.20.1 : les révisions 1 à 3 sont la fusion de l'amont, le routage MoE
// rétabli et le pilotage du pont (trois portes dans le menu, et le voisin piloté depuis
// sa carte avec le millésime de SON moteur). La révision 4 rend ce millésime lisible
// partout où il se décide : il AVERTIT au lieu de refuser le démarrage (un exe peut
// désormais servir plusieurs millésimes, le preset nomme déjà le sien), et chaque preset
// MoE porte une pastille VERTE ou ROUGE — verte quand le moteur posé est celui que le
// preset désigne ET celui qu'épinglette cette application. Plus deux corrections du
// contexte lui-même : un ordre de pilotage (stop, reprise, arrêt) ne repart plus dans
// les « constraints » du résumé, et la sortie des outils est signalée au modèle comme
// une DONNÉE à lire, non comme une consigne.
// Doit rester identique à
// cmd/ajean-memory-test/versioninfo.json (StringFileInfo.ProductVersion et
// FileVersion), vérifié par TestVersionFlavorWindowsSuit.
//
// Le .1 de ce soir (23/11, 23h40) est un correctif ECRIT A LA MAIN, pas une
// restauration : la barre des onglets se reveillait elle-meme, son observateur se
// declenchait sur SES PROPRES ecritures d'attributs (26-onglets.js, trois gardes).
// La révision .1 est l'ordre du 23/11 (« v1.0.1 »), qui répare le défaut nommé la
// veille : un +revN est INVISIBLE à la mise à jour, semver.Canonical efface le
// build-suffixe donc Compare(v1.0.0+rev0, v1.0) rend 0. Un incrément en « . » se
// compare. Le .1 porte aussi la bascule de l'encodeur mémoire : EmbeddingGemma 2
// passe du Python/Transformers (pic de 20 Go, 3,5 Go vifs en 2 min) au GGUF Q8 lu
// par llama.cpp, six fois plus petit et sans runtime Python.
const buildFlavorSuffix = ".2"

// uiBadgeLabel : intitulé affiché sous le logo par le build portable, à la
// place du repère « TEST mémoire · R6 ». Ce n'est plus « le nom du patch » :
// depuis le millésime 1.0.0, ce build n'est pas un correctif posé sur la base
// de l'amont, c'est la base (ordre de l'humain du 21/11 : « on est plus sur un
// patch correctif, mention à supprimer »). Le nom du patch ne disparaît pas
// pour autant : il vit dans l'étiquette git (+revN).
const uiBadgeLabel = "Aidros"

// uiBadgeMarker : la SIGNATURE du badge, pas son mot. Le garde de livraison
// doit pouvoir dire « le badge est posé » ou « il ne l'est pas ». Avant le
// renommage, il cherchait le mot nu (uiBadgeLabel) : le mot suffisait à lui
// seul à désigner le badge, puisqu'il n'apparaissait qu'là.
//
// Depuis que l'application s'appelle Aidros, ce mot est aussi dans <title>,
// dans le manifeste PWA, dans les libellés du pont, dans les chaînes i18n. Le
// mot nu ne distingue plus rien :
//
//	· le test positif passerait même si le badge disparaissait — un VERT QUI
//	  NE VÉRIFIE RIEN, le pire des états ;
//	· le test négatif (« badge absent du point d'entrée normal ») serait
//	  ROUGE sur une page parfaitement correcte.
//
// La balise, elle, est propre au badge : le <div id="memory-test-build"> du
// gabarit est « hidden », celui du badge porte un style. Si le balisage du
// badge change, le marqueur ne se trouve plus et le test positif vire rouge :
// le garde reste branché sur ce qu'il prétend garder.
const uiBadgeMarker = `<div id="memory-test-build" style=`

func displayVersion() string {
	if parallelBuild() {
		return Version + buildFlavorSuffix
	}
	return Version
}

func parallelBuild() bool                   { return BuildFlavor == "embedding-test" }
func parallelHome(executable string) string { return filepath.Join(filepath.Dir(executable), "data") }

// parallelHomeLabel est MORTHE le 22/11, et retirée avec, pas seulement son
// affichage. Ajoutée la veille pour que le badge nomme l'installation (le port
// et le dossier), elle n'a plus un seul appelant depuis que l'humain a demandé
// de ne garder que le lien du dépôt : « ne garde que le dépôt github ici, pas ce
// qui précède ». Vérifié au juge — « git grep parallelHomeLabel » ne rend plus
// que sa propre définition — donc ce n'est pas un routage perdu que je viens de
// casser, c'est bien le routage que l'humain a retiré : du code mort.
//
// L'information n'est pas perdue pour autant, et c'est ce qui autorise le
// retrait : le PORT est dans le <title> de la page et dans l'URL d'ouverture du
// navigateur, le DOSSIER dans « Emplacements » via /api/paths (handlePaths,
// sys_paths.go:126, qui rend home, database, exe, installed et six autres).
// Les rejouer si l'écran redevient avare : un commit du 22/11, 3a92bea.
//
// Le nom du dossier de données ne distinguait d'ailleurs RIEN : tout build
// portable pose son home à côté de son exécutable (parallelHome), donc « data »
// chez l'une comme chez l'autre. C'est le dossier qui les séparait qui parlait.

func initParallelProfile() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	// Intentionally ignore an inherited AJEAN_HOME pointing at production.
	if err = os.Setenv("AJEAN_HOME", parallelHome(exe)); err != nil {
		return err
	}
	if err = os.Setenv("AJEAN_SERVICE", "ajean-embedding-test"); err != nil {
		return err
	}
	for _, d := range dataDirs() {
		if err = os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	if len(ReadConfig()) == 0 {
		cfg := defaultConfig()
		cfg["MEM_MODE"] = "ondemand"
		if err = WriteConfig(cfg); err != nil {
			return err
		}
	}
	// Upgrade revision 1 in place: same DB/notes, fixed test-only listener and
	// an editable preset for the already working shared API connection.
	if err = WriteConfig(ReadConfig()); err != nil {
		return err
	}
	if presets, _ := ListPresets(); len(presets) == 0 && isExternalConfig(ReadConfig()) {
		if err = saveParallelSharedPreset(ReadConfig()); err != nil {
			return err
		}
	}
	var saved embeddingConfig
	if !getJSON(bkState, "memory_embeddings", &saved) {
		model := filepath.Join(AjeanHome(), "models", "embeddinggemma-2-Q8_0.gguf")
		_, e := os.Stat(model)
		binName := "llama-server"
		if strings.EqualFold(filepath.Ext(exe), ".exe") {
			binName += ".exe"
		}
		if err = saveEmbeddingConfig(embeddingConfig{
			Enabled: e == nil, Bin: filepath.Join(filepath.Dir(exe), "encoder", binName), Model: model, Port: 18081, Threads: 8,
		}); err != nil {
			return err
		}
	}
	// Migrate v1's default absolute paths and persist relative paths for moves.
	if err = saveEmbeddingConfig(readEmbeddingDefaults()); err != nil {
		return err
	}
	return nil
}

func parallelCommandAllowed(cmd string) bool {
	switch cmd {
	case "app", "web", "help", "-h", "--help", "version", "-v", "--version", "where", "serve", "import-presets", "bridge", "peer":
		return true
	// La clé API machine se définit déjà depuis l'interface (/api/apikey n'est
	// pas bloqué par parallelHTTPHandler). Sans elle, le pont Aidros↔Aidros ne
	// peut tout simplement pas exister : antKeyOK n'admet personne. La CLI
	// portable doit donc pouvoir la poser, comme l'UI le fait déjà.
	case "set-api-key":
		return true
	}
	return false
}

// parallelBlockedRoutes : ce que le build portable refuse, et POURQUOI. Une
// raison par préfixe : un message recopié pour six routes différentes rend un
// refus légitime incompréhensible. Mesuré le 2026-10-27 : qui cliquait sur
// « joignable depuis le réseau local » lisait « utiliser la page Mémoire
// sémantique », deux phrases sans aucun rapport avec le réseau.
var parallelBlockedRoutes = []struct {
	prefix string
	msg    string
}{
	{"/api/update/", "Mise à jour désactivée : cette version portable de Cresius se met à jour à la main (le binaire est livré avec son patch)."},
	{"/api/network", "Exposition du moteur sur le réseau désactivée dans cette version portable : le seul port ouvert vers l'extérieur est celui du pont Aidros↔Aidros (page Réglages, carte « Pont Aidros ↔ Aidros »), verrouillé par la clé API de la machine. Le port du moteur reste sur cette machine."},
	{"/api/cloud/", "GPU cloud désactivé dans cette version portable."},
}

func parallelHTTPHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if p == "/api/update" {
			sendJSON(w, 200, map[string]any{"current": Version, "available": false, "parallel_test": true})
			return
		}
		if strings.HasPrefix(p, "/api/moe/") && runtime.GOOS != "windows" {
			sendJSON(w, 403, map[string]any{"ok": false, "error": "MoE portable : ce chemin d’installation est réservé à Windows x64."})
			return
		}
		// Presets and llama.cpp management use their normal handlers. Services,
		// registry/network settings and updating this fork stay out of scope.
		//
		// Chaque refus porte SA raison. Un message unique recopié pour six préfixes
		// différents faisait répondre, à qui cliquait « joignable depuis le réseau
		// local », « utiliser la page Mémoire sémantique » — aucune des deux choses
		// n'a de rapport avec le réseau (mesuré le 2026-10-27, capture d'écran de
		// l'utilisateur). Le refus est légitime, l'explication doit l'être aussi.
		for _, b := range parallelBlockedRoutes {
			if strings.HasPrefix(p, b.prefix) {
				sendJSON(w, 403, map[string]any{"ok": false, "error": b.msg})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// uiOpenQueryParam : le MOT que l'URL d'ouverture affiche avant le millésime,
// dans la barre d'adresse (« …/?aidros-main=1.0.0 »).
//
// Il s'appelait « memory-test » depuis la preuve d'embeddings (l'époque où ce
// build servait à essayer un moteur de mémoire). L'application qu'il ouvre
// depuis s'appelle Aidros : l'adresse criait donc « test de mémoire » sous un
// titre « Aidros » — le même décalage que le raccourci « AJEAN » sous une
// infobulle « Aidros », mesuré le 22/11.
//
// ⚠ Ce n'est PAS une porte. Mesuré : `r.URL.Query()` n'est appelé nulle part
// sur l'interface, et le seul URLSearchParams du dépôt FABRIQUE un paramètre,
// il ne lit pas celui-ci. Le mot décore la barre
// d'adresse pour l'œil humain, et porte le millésime à côté. Donc le renommer
// ne casse aucune logique — mais trois fichiers reconstruisent la même chaîne
// (Lancer-Test.ps1:11, smoke_ui.py:74) et suivent ici.
//
// Le porteur de cet URLSearchParams s'appelait 13-remote.js ; il est parti le
// 22/11 avec « Ajean Link ». Le filet (url_ouverture_test.go) balaie
// désormais le DOSSIER et non une liste de noms, pour ne pas rougir sur la
// disparition d'un fichier — la leçon est dans ce test.
//
// ⚠ À NE PAS fondre avec l'IDENTIFIANT DU DOM du badge,
// « memory-test-build » (uiBadgeMarker ci-dessous) : ce n'est pas la même
// surface. Celui-là est gardé par parallel_delivery_test.go:20 et vit dans le
// gabarit ; il n'a pas bougé ce coup-ci.
const uiOpenQueryParam = "aidros-main"

// parallelOpenURL : l'adresse que le build portable ouvre dans le navigateur.
//
// Séparée de cmdParallelApp pour être JOUABLE. L'URL était construite en ligne
// dans la fonction qui ouvre le navigateur — donc aucune assurance ne pouvait
// dire ce qu'elle porte, et le nom qu'elle porte est justement ce qui a menti
// pendant un an sur l'identité de ce build.
//
// Elle reçoit l'ADRESSE d'écoute (« 127.0.0.1:18099 »), pas le port : c'est
// celle que net.Listen reçoit juste à côté. Écrire « 127.0.0.1 » en dur ici
// rendait l'URL fausse le jour où l'hôte d'écoute bougerait — un fichier qui
// dit d'aller là où le serveur ne s'est pas posé. Et 18090 n'est qu'un défaut
// (parallelUIPortDef) : AJEAN_UI_PORT décale, deux installations côte à côte se
// décalent. Écrire 18090 en dur dans un filet l'aurait rendu faux sur une
// instance décalée — mes scripts de diagnostic connaissent déjà 18090, 18094, 18095.
func parallelOpenURL(addr string) string {
	return "http://" + addr + "/?" + uiOpenQueryParam + "=" + displayVersion()
}

func cmdParallelApp() error {
	// Le port de l'interface se lit, il ne s'impose plus : deux installations
	// côte à côte (la vivante et une copie de réparation, ou les deux moitiés
	// d'un pont Aidros↔Aidros sur la même machine) doivent pouvoir démarrer sans
	// se voler 18090. AJEAN_UI_PORT ou la clé parallel_ui_port décalent.
	port := parallelUIPort()
	addr := "127.0.0.1:" + port
	url := parallelOpenURL(addr)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("port d’interface %s occupé ; aucun processus arrêté : %w (décaler avec AJEAN_UI_PORT=18094)", port, err)
	}
	defer ln.Close()
	defer stopEmbeddingServer()
	defer parallelServiceAction("stop")
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	go func() { <-signals; stopEmbeddingServer(); _ = parallelServiceAction("stop"); os.Exit(0) }()
	mux := newWebMux()
	srv := &http.Server{Handler: parallelHTTPHandler(gzipHandler(mux)), ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	if c := readEmbeddingConfig(); c.Enabled {
		root, _ := filepath.Abs(memoryDir())
		queueEmbeddingIndex(root, c)
	}
	// Le moteur doit annoncer le port qu'il écoute vraiment : la config porte
	// PORT, et tout ce qui parle au moteur en local passe par LLMPort().
	if err := ensureParallelEnginePort(); err != nil {
		fmt.Printf("[moteur] port non aligné : %v\n", err)
	}
	// Le pont Aidros↔Aidros s'allume ici, dans le process qui tient déjà la base :
	// la clé API est relue à chaque requête, donc une clé définie après le
	// démarrage est valable immédiatement. Éteint par défaut : rien ne s'expose
	// tout seul. La veille rattache l'écoute au réglage, pour qu'un
	// « aidros bridge on » lancé à côté de cette instance prenne sans redémarrer.
	bridgeStop := make(chan struct{})
	defer close(bridgeStop)
	if parallelBridgeEnabled() {
		if err := startParallelBridge(); err != nil {
			fmt.Printf("[pont aidros] non démarré : %v\n", err)
		}
	}
	go parallelBridgeWatch(bridgeStop)
	fmt.Printf("Aidros — %s\n%s\nDonnées : %s\nMoteur local : 127.0.0.1:%s ; API existante disponible par preset.\n", displayVersion(), url, AjeanHome(), parallelLocalEnginePort())
	_ = openBrowser(url) // normal browser tab: a separate profile is unnecessary
	runTray(url)
	return nil
}
