package aidros

// mem_embeddings_cache_disque.go — le cache des vecteurs survit au redémarrage.
//
// Le coût mesuré le 23/11 sur ce poste : 1448 fragments à réencoder à chaque
// lancement, 59 pages/min, soit ~25 minutes pendant lesquelles la recherche
// mémoire retombe sur le lexical. /api/embeddings le disait noir sur blanc :
// « cache : RAM uniquement (reconstruit au lancement) ». La capture de Julien
// l'a montré chiffré : 1448 fragments indexés, 0 recherche. Le prix avait été
// payé, sans avoir servi.
//
// Ce qui rend le gain possible existait déjà : rebuildEmbeddingIndex réutilise
// les vecteurs d'un index antérieur quand l'empreinte du moteur est la même
// (« old != nil && old.Fingerprint == client.Fingerprint »). Il ne manquait
// qu'un old qui survive au processus.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Le dossier du cache, sous le dossier mémoire du projet dont il est le cache.
// ⚠ Un SOUS-dossier, jamais un fichier posé à plat dans memory/ : le paquet de
// sauvegarde (buildBundleTar) ne déplie que les fichiers, jamais les dossiers, et
// le contrat « RAM-only » de mem_embeddings.go refuse que les vecteurs partent
// dans les sauvegardes. Surtout, trois parcours lisent memory/ en supposant qu'il
// est plat — mem_snapshots.go:49 le dit en toutes lettres, mem_migrate.go:57 et
// memListIn (chat_memory.go) sautent les dossiers, et restoreSnapshot ne retire
// que ce qui n'est pas un dossier (:124). Un fichier à plat se serait fait
// emporter par une restauration, ou prendre pour une page de mémoire.
//
// ⚠ Le nom n'est PAS « .cache » : ce nom est déjà refusé ailleurs dans le projet
// (apps_test.go:220, jean_files_test.go:97) et est sauté par le parcours des
// modèles portables (parallel_memory.go:51). Le nôtre est .memcache.
const dossierCacheMemoire = ".memcache"

// Le nom du fichier, et pourquoi pas « *.md » : une page de mémoire de plus
// aurait faussé indexFresh, qui compare le NOMBRE de documents, et son empreinte
// aurait changé à chaque indexation — la paire empreinte/vecteurs ne pourrait
// plus se retrouver d'un lancement à l'autre.
const fichierCacheMemoire = "embeddings.json"

// Le contrat posé sur le disque. Un cache dont le format change doit être jeté,
// pas deviné : c'est ce millésime qui le dit.
const millésimeCacheMemoire = "aidros-memoire-cache-1"

// cacheMemoireSurDisque est la forme écrite. Les champs d'embeddingIndex sont
// non exportés (donc ignorés par json.Marshal) : cette structure est le seul
// moyen de les écrire, et elle porte le millésime qui manque à l'original.
type cacheMemoireSurDisque struct {
	Millésime   string               `json:"millesime"`
	Fingerprint string               `json:"fingerprint"`
	Hashes      map[string]string    `json:"hashes"`
	Vectors     map[string][]float32 `json:"vecteurs"`
	Updated     time.Time            `json:"actualise"`
	// Anneau : le motif posé par le dernier écrivain — « index complet » quand
	// l'indexation a fini, « anneau après N fragments » quand elle s'est arrêtée
	// en chemin. Informatif pour l'interface ; le contrat de vérité n'est PAS là,
	// il est dans la CLÉ de chaque vecteur (voir anneauComplet).
	Anneau string `json:"anneau,omitempty"`
}

// anneauComplet est le motif que lireCacheMemoire lit pour reconstituer le
// drapeau embeddingIndex.Complet.
//
// ⭐ POURQUOI UN ANNEAU PARTIEL EST SÛR, et ce qui le prouve — la CLÉ, pas une
// promesse. memoryChunk.Key est memoryDigest(embeddingProtocol · fichier · titre
// · TEXTE · mediaHash) (mem_embeddings.go) : un vecteur rangé sous la clé K a
// donc été calculé sur le texte qui hache en K, ni plus ni moins. Un fragment
// rescapé d'un anneau ne peut pas être « un vecteur d'un autre texte » ; il est
// soit exactement son fragment, soit absent (la clé ne se retrouve plus).
//
// ➜ L'anneau peut être INCOMPLET. Il ne peut pas être FAUX. C'est ce qui rend
// possible de le sauver AVANT le juge de fraîcheur, sans jamais présenter un
// index comme à jour alors qu'il lui manque des fragments.
//
// anneauComplet est le motif de l'index terminé ; anneauAprès, celui d'un arrêt
// en cours de route — nommé par l'humain du 24/11, qui veut l'anneau mais pas le
// mensonge.
//
// ⭐ Le motif n'est pas un ornement : c'est LE drapeau de complétude. Écrire autre
// chose que anneauComplet déclare un anneau réutilisable mais non frais, et
// indexFresh le refuse (mem_embeddings.go:189). Il voyage avec le fichier : la
// complétude ne peut pas être déduite après coup, les empreintes de pages d'un
// anneau partiel étant, elles, complètes.
const anneauComplet = "index complet"

func anneauAprès(n int) string { return fmt.Sprintf("anneau après %d fragments", n) }

// cheminCacheMemoire ne prend PAS memoryDir() : le cache appartient au projet
// dont root est le dossier. memoryDir() suit le projet ACTIF, et une indexation
// en arrière-plan ne doit jamais écrire dans le dossier d'un autre projet.
func cheminCacheMemoire(root string) string {
	return filepath.Join(root, dossierCacheMemoire, fichierCacheMemoire)
}

// ecrireCacheMemoire pose l'index sur le disque. Le chiffrement suit la mémoire
// qu'il cache : encodeMemContent chiffre si MEM_ENCRYPTED est actif, et laisse en
// clair sinon — un cache en clair à côté d'une mémoire chiffrée serait une fuite
// que le contrat « RAM-only » refuse.
//
// ⚠ Appelé SOUS embeddingCache.Lock(). C'est un choix assumé : la section critique
// du cache n'est retenue que 30 ms de plus, une fois par indexation, et écrire
// hors du verrou aurait exigé de figer une copie de l'index (2,8 Mo de vecteurs)
// pour la confier à une goroutine. Un seul défaut à la fois.
//
// anneau nomme ce qui est posé : « index complet », ou « anneau après N
// fragments ». Un anneau incomplet est ÉCRIT comme tel, jamais comme un index
// terminé — sa sécurité vient des clés, pas de ce mot (voir anneauComplet).
func ecrireCacheMemoire(root string, idx *embeddingIndex, anneau string) error {
	if idx == nil || memLocked() {
		return nil // rien à écrire, ou mémoire verrouillée : la DEK est partie
	}
	if len(idx.Vectors) == 0 {
		return nil // un anneau vide n'est rien : il ne faut pas l'écrire, ni le relire comme un gain
	}
	plaine, err := json.Marshal(cacheMemoireSurDisque{
		Millésime:   millésimeCacheMemoire,
		Fingerprint: idx.Fingerprint,
		Hashes:      idx.Hashes,
		Vectors:     idx.Vectors,
		Updated:     idx.Updated,
		Anneau:      anneau,
	})
	if err != nil {
		return fmt.Errorf("cache mémoire en JSON : %w", err)
	}
	blob, err := encodeMemContent(plaine)
	if err != nil {
		return fmt.Errorf("chiffrement du cache mémoire : %w", err)
	}
	if err := memWriteFileAtomic(cheminCacheMemoire(root), blob, 0o600); err != nil {
		return fmt.Errorf("écriture du cache mémoire : %w", err)
	}
	return nil
}

// lireCacheMemoire retrouve l'anneau posé lors d'un lancement précédent. Renvoie
// nil sans erreur dans TOUS les cas où il n'y a rien à réutiliser : fichier
// absent, format d'un autre millésime, mémoire verrouillée (la DEK est partie),
// moteur qui a changé d'empreinte. Un cache qu'on ne comprend pas est jeté,
// jamais interprété de travers.
//
// ⭐ Il renvoie un ANNEAU, pas un « index à jour ». Un anneau peut être partiel :
// c'est voulu, et c'est sûr, parce que chaque vecteur est rangé sous la CLÉ de
// SON texte (voir anneauComplet). Ce qui interdit de le présenter comme frais,
// c'est le drapeau Complet reconstitué ci-dessous, plus indexFresh en dessous.
//
// ⚠ Le drapeau Complet est RECONSTITUÉ ICI depuis le motif posé par l'écrivain :
// seul anneauComplet lève le doute. Un anneau arrêté en chemin a des empreintes
// de pages COMPLÈTES — donc indexFresh, qui ne compare que des empreintes, le
// déclarerait frais avec des vecteurs manquants, et la recherche rendrait moins
// de résultats sans le dire. Le motif écrit est la seule vérité de la
// complétude : il voyage avec le fichier, il ne peut pas être déduit après coup.
func lireCacheMemoire(root, fingerprint string) *embeddingIndex {
	if memLocked() {
		return nil
	}
	blob, err := os.ReadFile(cheminCacheMemoire(root))
	if err != nil {
		return nil
	}
	plaine, err := decodeMemContent(blob)
	if err != nil {
		return nil // chiffré et clé absente, ou fichier abîmé : rien à reprendre
	}
	var cache cacheMemoireSurDisque
	if err := json.Unmarshal(plaine, &cache); err != nil {
		return nil
	}
	if cache.Millésime != millésimeCacheMemoire || cache.Fingerprint != fingerprint {
		return nil
	}
	if len(cache.Vectors) == 0 || cache.Hashes == nil {
		return nil
	}
	return &embeddingIndex{
		Fingerprint: cache.Fingerprint,
		Hashes:      cache.Hashes,
		Vectors:     cache.Vectors,
		Updated:     cache.Updated,
		Complet:     cache.Anneau == anneauComplet,
	}
}

// effacerCacheMemoire retire le fichier d'UN projet. Appelé quand le cache RAM
// est vidé pour une raison de fond — la mémoire vient d'être verrouillée, ou la
// configuration de l'encodeur a changé : un cache RAM vidé sans que le disque le
// soit rendrait le verrouillage de Julien temporaire.
func effacerCacheMemoire(root string) {
	if err := os.Remove(cheminCacheMemoire(root)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return
	}
}

// effacerCachesMemoireSurDisque fait le même ménage pour TOUS les projets.
// clearEmbeddingCache ne connaît pas de racine : il vide le cache RAM d'un coup,
// et le disque doit être vidé aussi d'un coup, sinon le projet qu'on ne regarde
// pas garde ses vecteurs sous une mémoire verrouillée.
//
// ⚠ projectsRoot(), pas memoryDir() : memoryDir() ne connaît que le projet ACTIF.
// C'est le même périmètre que allMemPageFiles (mem_migrate.go:70), qui chiffre
// toutes les pages parce que la DEK est unique — la raison est exactement la
// même ici.
func effacerCachesMemoireSurDisque() {
	roots, err := os.ReadDir(projectsRoot())
	if err != nil {
		return
	}
	for _, d := range roots {
		if !d.IsDir() {
			continue
		}
		effacerCacheMemoire(filepath.Join(projectsRoot(), d.Name()))
	}
}
