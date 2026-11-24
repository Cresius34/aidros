package aidros

// Le cache disque des vecteurs de mémoire : son filet.
//
// Le coût mesuré le 23/11 sur le poste de Julien : 1448 fragments réencodés à
// CHAQUE lancement, 59 pages/min, ~25 minutes. Et /api/embeddings le disait noir
// sur blanc : « cache : RAM uniquement (reconstruit au lancement) ». Sa capture
// l'a montré chiffré — 1448 fragments indexés, 0 recherche. Le prix avait été
// payé sans avoir servi.
//
// Ce que le correctif ajoute n'est pas la réutilisation : elle existait déjà
// (« old != nil && old.Fingerprint == client.Fingerprint »). Il manquait un
// « old » qui survive au processus. Le filet porte donc là-dessus, et sur les
// deux gardes qui rendent le disque inoffensif : le juge d'empreinte, et
// l'effacement quand la mémoire se verrouille.
//
// ⚠ Ce filet n'exerce PAS l'encodeur : il n'y a pas de llama-server factice dans
// ce paquet, et en monter un ferait dépendre la garde du disque d'un vrai
// GGUF. La couche disque est jouée à nu, le câblage est relu dans la source, et
// le gain (zéro appel à l'encodeur après redémarrage) se mesure sur l'instance
// en service — pas ici.

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// cacheDeTest pose un index d'un fragment, avec l'empreinte voulue.
//
// ⚠ Complet: true — depuis le 24/11, indexFresh exige le drapeau : un anneau
// partiel a de bonnes empreintes et des vecteurs manquants, il n'est PAS frais.
// Une fixture qui l'omettrait ne prouverait que le refus.
func cacheDeTest(fp string) *embeddingIndex {
	return &embeddingIndex{
		Fingerprint: fp,
		Hashes:      map[string]string{"fiche.md": "empreinte-de-page"},
		Vectors:     map[string][]float32{"fiche.md:0-8": {0.5, 0.25}},
		Updated:     time.Now().UTC(),
		Complet:     true,
	}
}

func TestCacheMemoireSurDisque(t *testing.T) {
	testHome(t)
	addMemPage(t, "fiche.md", "# Fiche\n\nLe grenier garde les outils.")
	root, err := filepath.Abs(memoryDir())
	if err != nil {
		t.Fatal(err)
	}
	fp, err := embeddingFingerprint(readEmbeddingConfig())
	if err != nil {
		t.Logf("empreinte du moteur en service indisponible : %v", err)
		fp = "empreinte-de-test"
	}
	chemin := filepath.Join(root, dossierCacheMemoire, fichierCacheMemoire)

	// 1. écrire puis relire, en mémoire claire.
	if err := ecrireCacheMemoire(root, cacheDeTest(fp), anneauComplet); err != nil {
		t.Fatalf("écriture en clair : %v", err)
	}
	fi, err := os.Stat(chemin)
	if err != nil {
		t.Fatalf("le cache n'est pas posé sous %s/ : %v", dossierCacheMemoire, err)
	}
	if fi.Size() < 40 {
		t.Fatalf("le cache fait %d octets, attendu une vraie charge de vecteurs", fi.Size())
	}
	lu := lireCacheMemoire(root, fp)
	if lu == nil {
		t.Fatal("le cache posé à l'instant n'est pas relisible")
	}
	if len(lu.Vectors) != 1 || len(lu.Hashes) != 1 {
		t.Errorf("vecteurs retrouvés : %d, empreintes : %d, attendu 1 et 1", len(lu.Vectors), len(lu.Hashes))
	}
	if lu.Fingerprint != fp {
		t.Errorf("l'empreinte relue ne correspond pas : %q", lu.Fingerprint)
	}

	// 2. le dossier mémoire n'a pas enflé d'une page : le contrat
	// « memory/ est plat » (mem_snapshots.go:49) doit tenir, sinon le cache
	// deviendrait une page de mémoire et fausserait indexFresh.
	if _, err := os.Stat(filepath.Join(root, fichierCacheMemoire)); err == nil {
		t.Error("un fichier de cache est posé À PLAT dans le dossier mémoire : il serait pris pour une page")
	}
	// memListIn, pas mdPages : memListIn écarte MEMORY.md (l'index du projet)
	// comme il écarte les dossiers. mdPages compte MEMORY.md et annonçait « 2
	// pages » pour une seule — un rouge pour une raison sans rapport.
	if n := len(memListIn(root)); n != 1 {
		t.Errorf("%d pages sous le projet, attendu 1 : le cache est venu s'y ajouter", n)
	}
	// Il ne part pas dans les sauvegardes : buildBundleTar ne déplie que les
	// fichiers d'un dossier, jamais les dossiers.
	if strings.HasPrefix(fichierCacheMemoire, "MEMORY") {
		t.Error("le cache porte le nom de l'index du projet")
	}

	// 3. le juge d'empreinte : un moteur qui change ne doit PAS resservir ces
	// vecteurs, même parfaitement relisibles.
	if lu := lireCacheMemoire(root, "empreinte-d-un-autre-moteur"); lu != nil {
		t.Error("le cache d'un autre moteur est accepté : des vecteurs étrangers entreraient dans l'index")
	}

	// 4. mémoire verrouillée : le cache devient ILLISIBLE, et rien de neuf n'est
	// écrit. ⚠ Ce que la mesure a montré, et que mon premier jet ignorait : passer
	// MEM_ENCRYPTED à « 1 » fait partir le fichier — le cache en clair ne survit
	// pas à l'activation du chiffrement. C'est ce qu'on veut (un clair qui traîne
	// sous une mémoire chiffrée est la fuite), mais ce n'est pas le contrat de CE
	// sous-test : ici on garde le fichier, et on vérifie qu'il ne sert plus à rien.
	if err := SetConfigKey("MEM_ENCRYPTED", "1"); err != nil {
		t.Fatal(err)
	}
	clearMemDEK()
	if !memLocked() {
		t.Fatal("la mémoire devrait être verrouillée")
	}
	_, survit := os.Stat(chemin)
	t.Logf("le cache en clair a survécu à l'activation du chiffrement : %v", survit == nil)
	if lu := lireCacheMemoire(root, fp); lu != nil {
		t.Error("le cache est relisible alors que la mémoire est verrouillée")
	}
	if err := ecrireCacheMemoire(root, cacheDeTest(fp), anneauComplet); err != nil {
		t.Errorf("écriture sous mémoire verrouillée : %v", err)
	}
	if lu := lireCacheMemoire(root, fp); lu != nil {
		t.Error("écrire sous mémoire verrouillée a rendu le cache relisible")
	}
	if err := SetConfigKey("MEM_ENCRYPTED", ""); err != nil {
		t.Fatal(err)
	}

	// 5. verrouiller vide le cache : le disque doit SUIVRE le RAM, sinon le
	// verrouillage n'est que temporaire et la recherche retrouverait ses vecteurs.
	// ⚠ Reposé exprès : le fichier peut être parti à l'étape 4, et ce qu'on garde
	// ici est l'effacement par clearEmbeddingCache, pas ce qui l'a précédé.
	if err := ecrireCacheMemoire(root, cacheDeTest(fp), anneauComplet); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(chemin); err != nil {
		t.Fatalf("le cache n'est pas reposé avant le ménage : %v", err)
	}
	embeddingCache.Lock()
	embeddingCache.Indexes[root] = cacheDeTest(fp)
	embeddingCache.Unlock()
	clearEmbeddingCache()
	if _, err := os.Stat(chemin); err == nil {
		t.Error("le cache disque survit à clearEmbeddingCache : la mémoire verrouillée n'aurait été que temporaire")
	}
	if lu := lireCacheMemoire(root, fp); lu != nil {
		t.Error("le cache effacé est encore relu")
	}

	// 6. le ménage couvre tous les projets, pas seulement l'actif : la DEK est
	// unique, un projet verrouillé ne doit pas garder ses vecteurs.
	ailleurs := filepath.Join(projectsRoot(), "autre-projet")
	if err := os.MkdirAll(filepath.Join(ailleurs, dossierCacheMemoire), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ailleurs, dossierCacheMemoire, fichierCacheMemoire), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	effacerCachesMemoireSurDisque()
	if _, err := os.Stat(filepath.Join(ailleurs, dossierCacheMemoire, fichierCacheMemoire)); err == nil {
		t.Error("un projet non actif garde son cache après le ménage : sa mémoire verrouillée ne serait que temporaire")
	}

	// 7. ce qui est écrit est bien ce qui a été voulu, dans les deux sens — en
	// clair, puis chiffré, puisque le chiffrement suit la mémoire.
	plaine := []byte(`{"millesime":"` + millésimeCacheMemoire + `","vecteurs":{"a":[1,0]}}`)
	for _, chiffre := range []bool{false, true} {
		if err := SetConfigKey("MEM_ENCRYPTED", boolChiffre(chiffre)); err != nil {
			t.Fatal(err)
		}
		if chiffre {
			// Le patron de TestMemDEKState (mem_vault_test.go:110) : une DEK au
			// hasard suffit pour chiffrer, inutile de monter un vrai coffre.
			dek, err := randBytes(memDEKLen)
			if err != nil {
				t.Fatalf("DEK pour chiffrement : %v", err)
			}
			setMemDEK(dek)
		} else {
			clearMemDEK()
		}
		blob, err := encodeMemContent(plaine)
		if err != nil {
			t.Fatalf("chiffrer (%v) : %v", chiffre, err)
		}
		if chiffre && string(blob) == string(plaine) {
			t.Error("le cache est passé en clair alors que le chiffrement est actif")
		}
		back, err := decodeMemContent(blob)
		if err != nil {
			t.Fatalf("relire (%v) : %v", chiffre, err)
		}
		if string(back) != string(plaine) {
			t.Errorf("le cache relis ne rend pas ce qui a été écrit (chiffre=%v)", chiffre)
		}
		clearMemDEK()
	}
	if err := SetConfigKey("MEM_ENCRYPTED", ""); err != nil {
		t.Fatal(err)
	}

	// 8. un cache qu'on ne comprend pas est jeté, jamais interprété de travers.
	if err := os.MkdirAll(filepath.Dir(chemin), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, ordure := range []string{"", "pas du json",
		`{"millesime":"un-autre"}`,
		`{"millesime":"` + millésimeCacheMemoire + `"}`,
		`{"millesime":"` + millésimeCacheMemoire + `","fingerprint":"` + fp + `","hashes":{},"vecteurs":{}}`} {
		if err := os.WriteFile(chemin, []byte(ordure), 0o600); err != nil {
			t.Fatal(err)
		}
		if lu := lireCacheMemoire(root, fp); lu != nil {
			t.Errorf("une charge illisible (%q) a été acceptée", ordure)
		}
	}

	// 9. les deux noms sont gardés : le dossier n'est pas celui que le projet
	// refuse déjà (apps_test.go:220, jean_files_test.go:97, parallel_memory.go:51),
	// et le fichier ne porte pas l'extension d'une page de mémoire.
	if dossierCacheMemoire == ".cache" {
		t.Error("le dossier du cache est « .cache » : ce nom est déjà refusé par trois endroits du projet")
	}
	if filepath.Ext(fichierCacheMemoire) == ".md" {
		t.Error("le cache porte une extension .md : il serait chargé comme une page de mémoire")
	}
	if !strings.HasPrefix(dossierCacheMemoire, ".") {
		t.Error("le dossier du cache n'est pas caché : il serait listé avec les projets")
	}

	// 10. la chaîne qui mentait est partie, et l'interface rend un fait.
	if contains(sourceIndexation(t), "RAM uniquement (reconstruit au lancement)") {
		t.Error("la chaîne « RAM uniquement (reconstruit au lancement) » est encore dans mem_embeddings.go")
	}
	web := sourceWebEmbeddings(t)
	if contains(web, "RAM uniquement (reconstruit au lancement)") {
		t.Error("la chaîne « RAM uniquement (reconstruit au lancement) » est encore rendue à l'interface")
	}
	if !contains(web, "cheminCacheMemoire") {
		t.Error("l'interface ne consulte pas le disque : elle rend une promesse au lieu d'un fait")
	}
}

// Le cache doit être relisible par le moteur en place : l'empreinte est la seule
// clé. indexFresh compare des empreintes — il n'a donc besoin d'aucun GGUF posé.
// Mon premier jet appelait embeddingFingerprint et sautait le sous-test quand
// aucun moteur n'était en place : le juge n'était joué QUE DEPUIS L'ATELIER,
// jamais dans la suite. Un filet qui se met en congé tout seul ne garde rien.
func TestCacheMemoireSurDisquePasseLeJuge(t *testing.T) {
	testHome(t)
	addMemPage(t, "fiche.md", "# Fiche\n\nLe grenier garde les outils.")
	root, err := filepath.Abs(memoryDir())
	if err != nil {
		t.Fatal(err)
	}
	const fp = "empreinte-du-moteur-en-place"

	// L'empreinte du fragment doit être celle que loadMemoryDocumentsFor produit,
	// sinon indexFresh rejette pour une raison sans rapport. On la lui demande.
	docs, err := loadMemoryDocumentsFor(root, readEmbeddingConfig())
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 {
		t.Fatalf("%d documents chargés, attendu 1 : le juge n'aurait rien à juger", len(docs))
	}
	if err := ecrireCacheMemoire(root, &embeddingIndex{
		Fingerprint: fp,
		Hashes:      map[string]string{docs[0].File: docs[0].Hash},
		Vectors:     map[string][]float32{docs[0].File + ":0-8": {0.5, 0.25}},
		Updated:     time.Now().UTC(),
		Complet:     true,
	}, anneauComplet); err != nil {
		t.Fatal(err)
	}
	lu := lireCacheMemoire(root, fp)
	if lu == nil {
		t.Fatal("cache illisible")
	}
	if !indexFresh(lu, docs, fp) {
		t.Error("indexFresh rejette le cache relis du disque, alors qu'il sort de l'indexation en cours")
	}
	// Une page qui change doit chasser le cache : le juge ne dort pas.
	addMemPage(t, "nouvelle.md", "# Nouvelle\n\nUne page de plus, le cache ne la connaît pas.")
	docs2, err := loadMemoryDocumentsFor(root, readEmbeddingConfig())
	if err != nil {
		t.Fatal(err)
	}
	if indexFresh(lu, docs2, fp) {
		t.Error("indexFresh accepte un cache qui ignore une page ajoutée depuis : des vecteurs manquants seraient présentés comme à jour")
	}
	// Et l'empreinte du moteur reste la clé : le même contenu sous un autre
	// moteur ne doit pas être déclaré frais.
	if indexFresh(lu, docs, "empreinte-d-un-autre-moteur") {
		t.Error("indexFresh déclare frais un index dont l'empreinte de moteur ne correspond pas")
	}
}

// Le câblage, relu dans la source. Sans les trois branchements, le fichier ne
// sert à rien : c'est ce que ce filet garde, et rien de plus. Borné au corps de
// chaque fonction — cherché dans tout le fichier, un garde se satisferait de la
// définition de la fonction elle-même.
func TestCacheMemoireSurDisqueEstBranche(t *testing.T) {
	t.Run("l'indexation relit le disque quand le RAM est vide", func(t *testing.T) {
		corps := corpsDeFonction(t, "mem_embeddings.go", "rebuildEmbeddingIndex")
		if !contains(corps, "lireCacheMemoire(root, client.Fingerprint)") {
			t.Error("l'indexation ne consulte pas le disque : un redémarrage repart à zéro")
		}
		if !contains(corps, "ecrireCacheMemoire(root, idx,") {
			t.Error("l'indexation ne pose jamais son index sur le disque")
		}
	})

	// ⭐ LE GARDE DU 24/11 : l'anneau est posé AVANT que le juge de fraîcheur
	// jette. Le défaut, mesuré : ecrireCacheMemoire ne vivait qu'APRÈS ce juge,
	// donc sur un dossier qui bouge (et il bouge — le ménage de 05 h édite les
	// pages) le `return` rendait le cache disque inatteignable PAR CONSTRUCTION.
	// /api/embeddings répondait « RAM ; aucun cache disque » en permanence.
	//
	// Le garde est borné au TRONCON entre le juge et son message : chercher
	// « sauveAnneau » dans tout le corps se satisferait d'une sauvegarde posée
	// n'importe où ailleurs, y compris après — ce qui est exactement le défaut.
	t.Run("l'anneau est posé AVANT que le juge de fraîcheur jette l'index", func(t *testing.T) {
		corps := corpsDeFonction(t, "mem_embeddings.go", "rebuildEmbeddingIndex")
		i := strings.Index(corps, "!indexFresh(idx, current,")
		if i < 0 {
			t.Fatal("le juge de fraîcheur n'est plus dans l'indexation : le garde n'a plus de prise")
		}
		j := strings.Index(corps[i:], "fichiers modifiés pendant")
		if j < 0 {
			t.Fatal("le message du juge a changé : relire ce garde avant de conclure")
		}
		troncon := corps[i : i+j]
		if !strings.Contains(troncon, "sauveAnneau(") {
			t.Error("aucune sauvegarde entre le juge et son abandon : le cache disque redevient inatteignable sur un dossier qui bouge")
		}
	})

	// L'anneau doit aussi survivre aux quatre autres sorties de la boucle : le
	// délai, l'annulation, l'erreur d'encodage, la panne d'encodeur. Chacune
	// jetait le travail de dizaines de minutes, mesuré le 23/11 (1316/1448).
	t.Run("l'anneau est posé à chaque sortie de la boucle d'encodage", func(t *testing.T) {
		corps := corpsDeFonction(t, "mem_embeddings.go", "rebuildEmbeddingIndex")
		debut := strings.Index(corps, "for i, c := range chunks")
		fin := strings.Index(corps, "current, err := loadMemoryDocumentsFor")
		if debut < 0 || fin < 0 || fin < debut {
			t.Fatal("la boucle d'encodage n'est plus reconnaissable : le garde n'a plus de prise")
		}
		boucle := corps[debut:fin]
		if n := strings.Count(boucle, "sauveAnneau("); n < 4 {
			t.Errorf("%d sauvegarde(s) d'anneau dans la boucle, attendu au moins 4 (délai, annulation, encodage, encodeur)", n)
		}
		if !strings.Contains(boucle, "anneauTousFragments") {
			t.Error("l'anneau n'est plus sauvé PENDANT la route : il ne protège plus d'une coupure en cours d'indexation")
		}
	})

	t.Run("la recherche relit le disque AVANT de retomber sur le lexical", func(t *testing.T) {
		corps := corpsDeFonction(t, "mem_embeddings.go", "SearchMemory")
		i := strings.Index(corps, "lireCacheMemoire(root, client.Fingerprint)")
		if i < 0 {
			t.Fatal("la recherche ne consulte pas le disque : c'est pourtant le gain visible")
		}
		// ⚠ Marqueur de CODE, pas de prose : mon premier garde cherchait
		// « index absent ou modifié », et MA PROPRE PHRASE DE COMMENTAIRE le
		// contenait avant l'appel au disque — le garde rugissait sur le
		// correctif. fallback("... est ce que le compilateur voit.
		j := strings.Index(corps, `fallback("index absent`)
		if j >= 0 && i > j {
			t.Error("le disque est consulté APRÈS le repli lexical : la recherche retomberait sur le lexical comme le 23/11")
		}
	})

	t.Run("verrouiller la mémoire emporte le cache disque", func(t *testing.T) {
		corps := corpsDeFonction(t, "mem_embeddings.go", "clearEmbeddingCache")
		if !contains(corps, "effacerCachesMemoireSurDisque()") {
			t.Error("clearEmbeddingCache ne touche pas le disque : le verrouillage ne durerait qu'un redémarrage")
		}
	})

	t.Run("le cache appartient au projet qu'on indexe, pas au projet actif", func(t *testing.T) {
		// memoryDir() suit le projet ACTIF. Une indexation en arrière-plan qui
		// l'utiliserait écrirait le cache d'un projet dans le dossier d'un autre.
		corps := corpsDeFonction(t, "mem_embeddings_cache_disque.go", "cheminCacheMemoire")
		if contains(corps, "memoryDir()") {
			t.Error("le chemin du cache passe par memoryDir() : il suivrait le projet actif, pas celui qu'on indexe")
		}
		if !contains(corps, "root") {
			t.Error("le chemin du cache n'est pas construit depuis la racine du projet indexé")
		}
	})

	t.Run("le ménage du disque couvre tous les projets", func(t *testing.T) {
		corps := corpsDeFonction(t, "mem_embeddings_cache_disque.go", "effacerCachesMemoireSurDisque")
		if !contains(corps, "projectsRoot()") {
			t.Error("le ménage ne parcourt que le projet actif : un projet verrouillé garderait ses vecteurs")
		}
	})

	t.Run("l'interface rend ce qui est sur le disque", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handleEmbeddingSettings(rec, httptest.NewRequest("GET", "/api/embeddings", nil))
		if rec.Code != 200 {
			t.Fatalf("code %d", rec.Code)
		}
		var corps map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &corps); err != nil {
			t.Fatalf("décoder la réponse : %v", err)
		}
		c, ok := corps["cache"].(string)
		if !ok || c == "" {
			t.Fatalf("la clé « cache » est absente ou vide : %v", corps["cache"])
		}
		// La chaîne rendue est construite par le code qu'on garde, quel que soit
		// l'état du moteur : ces deux gardes ne dépendent donc d'aucun GGUF posé.
		if strings.Contains(c, "uniquement") || strings.Contains(c, "reconstruit") {
			t.Errorf("l'interface rend encore une promesse, pas un fait : %q", c)
		}
		if !strings.HasPrefix(c, "RAM") {
			t.Errorf("l'interface ne dit pas ce qu'elle a regardé : %q", c)
		}
		// Ce qui est vérifié sur le disque doit être dit : présent et nommé, ou
		// explicitement absent. Un « RAM » sec serait encore une promesse.
		chemin := filepath.Join(memoryDir(), dossierCacheMemoire, fichierCacheMemoire)
		_, posé := os.Stat(chemin)
		if posé == nil && !strings.Contains(c, dossierCacheMemoire) {
			t.Errorf("le cache est sur le disque mais l'interface ne le nomme pas : %q", c)
		}
		if posé != nil && !strings.Contains(c, "aucun") {
			t.Errorf("aucun cache sur le disque mais l'interface en annonce un : %q", c)
		}
	})
}

// Les deux auxiliaires de ce filet.
func boolChiffre(b bool) string {
	if b {
		return "1"
	}
	return ""
}

func sourceWebEmbeddings(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("web_embeddings.go")
	if err != nil {
		t.Fatalf("relire web_embeddings.go : %v", err)
	}
	return string(b)
}

// corpsDeFonction relit le corps d'une fonction, de son { d'ouverture à
// l'accolade fermante en début de ligne. Borné exprès : un garde cherché dans
// tout le fichier se satisferait de la définition de la fonction qu'il garde.
func corpsDeFonction(t *testing.T, fichier, fonction string) string {
	t.Helper()
	b, err := os.ReadFile(fichier)
	if err != nil {
		t.Fatalf("relire %s : %v", fichier, err)
	}
	src := string(b)
	i := strings.Index(src, "func "+fonction+"(")
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
