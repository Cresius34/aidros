# Aidros `1.1.0` — le système d'embeddings EmbeddingGemma 2, refondu

Un cran de **fonction** : l'encodeur de mémoire change de voie, sa mémoire de
 travail survit au redémarrage, et l'interface arrête d'attendre le moteur.
 Quatre coûts mesurés, quatre correctifs, chacun avec un filet qui mord.

**Projet dérivé, usage personnel.** Construit sur [**nathaninline/ajean**](https://github.com/nathaninline/ajean)
(projet principal, MIT), dont il descend en `0.20.1`. Les changements décrits ici
forment une **ligne privée** : non validés sur d'autres machines, non garantis
transposables. Pour installer AJEAN, prenez les versions publiées sur
[nathaninline/ajean](https://github.com/nathaninline/ajean/releases/latest).

## Les gains, chiffrés

| | avant | après |
|---|---|---|
| **RAM privée de l'encodeur** | **5,21 Go** | **1,27 Go** (0,27 Go vifs) |
| **Indexation de la mémoire** | **~25 min à CHAQUE lancement** | **une seule fois** |
| **Réserve de hauteur de l'interface** | **2 402 ms** | **29 à 75 ms** par bloc |

Le débit d'indexation mesuré est de **59 pages/minute** (1 310 → 1 436 en 60 s),
sur 1 448 fragments. C'est ce débit qui faisait les ~25 minutes — et c'est surtout
le fait que le cache ne survit pas au processus qui les faisait **repayer à chaque
démarrage**.

## 1. L'encodeur passe de Python/Transformers à llama.cpp/GGUF

L'encodeur d'embeddings tournait sur Python/Transformers : il mangeait la machine,
et son veilleur a fini par tuer **pour inactivité une instance déjà morte**. Le
job d'indexation a reçu une erreur réseau au lieu d'une relance.

Il tourne maintenant sur **llama.cpp/GGUF** : `embeddinggemma-2-Q8_0.gguf`,
`device=cpu`, un seul fichier de 310 Mio contre tout un dossier de poids. Les
vecteurs rendus sont **les mêmes** — l'épreuve a été jouée sur les deux voies, pas
supposée. Prêt en **2 s**, `POST /v1/embeddings` répond en **0,09 s**.

⚠ **Le python de 73 Go n'est pas l'encodeur.** C'est le **moteur MoE** de
conversation (ses poids d'experts relus par PCIe, le journal dit « expert cache
95,1 % hit »). Il existait avant cette bascule et n'y change rien. **Ce n'est pas
de la VRAM**, contrainte à l'intuition : c'est de la RAM.

⚠ **Correction d'une affirmation de la note `1.0.0`**, qui annonçait
« EmbeddingGemma-2 (256 dimensions) ». Sur ce poste, la configuration vivante rend
**`dimensions: 512`**, et le code attend **768** pour EmbeddingGemma 2 quand il
valide une réponse. La dimension est **réglable par preset** (768 / 512 / 256 /
128) : elle n'aurait pas dû être écrite comme une constante du produit.

## 2. Le cache des vecteurs survit au redémarrage

C'était le coût le plus visible et le moins nommé : l'interface le disait pourtant
noir sur blanc, « cache : **RAM uniquement** (reconstruit au lancement) ». Le prix
avait donc été payé, sans avoir servi : 1 448 fragments indexés pour **0
recherche**.

Le cache vit sous `<racine du projet>/.memcache/embeddings.json` (2,8 Mo pour
1 448 fragments).

- Il est **chiffré** quand la mémoire l'est, et **verrouiller la mémoire l'emporte
  pour tous les projets** : la clé est unique, un projet verrouillé ne doit pas
  garder ses vecteurs en clair. Sans cela, le verrouillage n'aurait duré qu'un
  redémarrage.
- Le **juge d'empreinte tient** : un moteur qui change n'exporte pas ses vecteurs,
  on ne mélange pas deux espaces vectoriels.
- `SearchMemory` relit le disque **avant** le repli lexical. C'est le branchement
  qui rend le gain visible : sans lui, le fichier n'aurait servi qu'à l'indexation
  et la recherche serait retombée sur « index absent ou modifié ».
- Un fichier **à plat** dans `memory/` l'aurait faussée : la fraîcheur de l'index se
  juge en comparant le **nombre** de documents. D'où le sous-dossier, et le nom
  `.memcache` (`.cache` est déjà refusé par trois endroits du projet).

## 3. L'interface ne rend plus une promesse

`/api/embeddings` affichait une chaîne, pas un fait. Elle rend maintenant ce qui
est sur le disque à l'instant : « RAM + disque (.memcache/embeddings.json,
N octets) », ou « RAM ; aucun cache disque ».

## 4. Le tchat n'est plus otage du moteur

Les six panneaux du réglage gardaient une hauteur réservée **jusqu'à ce que les
dix-neuf chargeurs aient fini**. Or le plus lent — `/api/moe` à **2,396 s**, les
dix-huit autres à **0,079 s ou moins** — ne peint **aucun** des six : il les
retenait tous, pour une raison sans rapport avec eux.

Chaque panneau rend la main dès que **le chargeur qui le peint** a fini, sur les
deux issues (un échec rend la main lui aussi : c'est précisément le cas « accès
distant coupé »). Chaque chargeur n'est appelé **qu'une fois** : relancer une
promesse pour attendre en ferait deux requêtes.

Le mural de chargement ne bouge pas (2 402 → 2 404 ms) : ce qui change est le
**moment où l'écran rend la main**, pas la durée du chargement.

## 5. L'indexation se reprend elle-même

Une erreur réseau sur l'encodeur **redémarre et reprend le job au point d'arrêt**,
au lieu d'afficher une panne. Mesuré : **1 316/1 445 → 1 445/1 445**. La première
mort de l'encodeur reste non élucidée (le journal finit net, sans message de
sortie) ; ce qui était prouvé et mien, c'est que **le job ne repartait jamais**.

## Comment c'est prouvé

Chaque correctif a son filet, et la morsure est **jouée dans les deux sens** :

- `mem_embeddings_cache_disque_test.go` — trois niveaux : la couche disque à nu,
  la fraîcheur de l'index jouée avec des empreintes fabriquées, et le câblage relu
  dans la source. **Cinq mutations sur cinq** l'ont fait rougir, avec la raison
  nommée, restauration à l'empreinte identique à chaque fois.
- `ui_reserve_hauteur_test.go` — lit **tout** le JS, qui est éclaté en dix-huit
  fichiers (`loadCfg` vit dans `05-status.js`, `loadMCP` dans `12-mcp.js`).
  **Huit mutations sur huit** l'ont fait rougir.

Deux « verts » de la morsure étaient **FAUX** : mes mutations n'atteignaient pas
le fichier (j'écrivais le motif `mcp-list:'loadMCP'` là où le fichier porte
`'mcp-list':'loadMCP'`, clé **quotée**). Trouvé en **comptant les occurrences**,
pas en raisonnant. Un filet qui ne trouve rien peut passer pour un filet qui
garde bien.

Quatre autres défauts de mon code ont été trouvés par mes filets et non par la
lecture : un auxiliaire qui comptait l'index du projet parmi les pages de
mémoire ; un garde d'ordre qui lisait **ma propre phrase de commentaire** ;
`%d` passé à une chaîne ; et un auxiliaire Go (`corpsDeFonction`, qui cherche le
mot `func`) réutilisé sur du JS — il ne trouve jamais rien, et un filet qui ne
trouve rien passe pour un filet qui garde bien.

`gofmt -l` muet sur le dépôt, `go vet ./...` 0 ligne, suite complète ok en
140,9 s. Le binaire est relu **dans ses octets, dans les deux sens** : les marques
des deux correctifs sont absentes du 1.0.1 et présentes dans le 1.1.0, et le
mensonge « RAM uniquement » est présent dans le vieux, **absent** du neuf.

## Ce qui n'est pas là, dit franchement

- **Le premier lancement sous 1.1.0 paie la toute première réindexation sous
  llama.cpp** : une fois, plus jamais après.
- **La bascule de l'encodeur est un choix du preset, pas du binaire.** Ce
  millésime la rend possible et la rend visible ; il ne l'impose pas.
- **Les minuteries de l'interface ne sont pas touchées.** Quatre tournent sous la
  seconde (200, 250, 500 ×2, 800 ms), dont une qui fait un `sync()` **réseau** tous
  les 500 ms. C'est un autre chantier, et la cause mesurée du « chargement long »
  des panneaux était la réserve de hauteur, pas elles.
- **Le premier rendu du fil ne page pas.** Un fil de 148 108 jetons est rejoué en
  entier ; la pagination existe (`/api/chat/older` répond en 0,014 s) et le premier
  rendu ne s'en sert pas. C'est le plus gros des trois gains restants.
- La génération d'images n'est toujours pas codée ; pas de coffre à secrets, pas de
  sauvegarde ; les correctifs de l'amont ne tomberont plus tout seuls (leur `main`
  est un historique **sans ancêtre commun** avec le nôtre).

## État sur le poste de fabrication

Le binaire est **posé et libre** aux deux endroits attendus (racine `aidros.exe`
et `data\bin\aidros.exe`, 17 978 880 octets / `C59B0183DF4B` / identité Windows
**1.1.0**), avec son point de retour (`aidros.avant-1.1.0.bak.exe`, le 1.0.1).
**Rien lancé, rien tué** : l'instance en service tourne encore sur le 1.0.0.
L'étiquette `v1.1.0` n'est **pas encore posée** : la publication attend un accord
explicite.

## Ce que cette version épingle

Les trois millésimes que le binaire refuse ou accepte, posés dans le code et
vérifiés par le préflight au démarrage :

| | version |
|---|---|
| **moteur MoE Strata** | **0.1.59** (paquet `strata-engine-0.1.59-…-hip-rx9070`) |
| **panneau de contrôle** | **0.1.54** |
| **source Strata vendue** | **0.2.8** |
| **encodeur d'embeddings** | `embeddinggemma-2-Q8_0.gguf`, **309 855 456 octets**, llama.cpp b11669 |

⚠ **Un millésime est épinglé PAR POSTE, pas une valeur unique** : la variante sur
RX 6900 XT (gfx1030) vise une ligne RDNA2 distincte. C'est ce qui, le 15/11,
faisait refuser à un exe épinglé 0.1.57 son propre vieux preset 0.1.48 **quel que
soit l'état du moteur** posé sur le disque.

## Ascendance

- Base amont : `nathaninline/ajean` **0.20.1** (MIT).
- Moteur MoE : [**Niko1221/Strata**](https://github.com/Niko1221/Strata) — **0.1.59**
  posé, **0.1.54** de contrôle, source **0.2.8**.
- Embeddings : [**google/embeddinggemma-2**](https://huggingface.co/google/embeddinggemma-2),
  servi par [**ggml-org/llama.cpp**](https://github.com/ggml-org/llama.cpp).
- Notes de la ligne précédente (`1.0.0`, `v1.0.0+rev0`) :
  [docs/notes-de-version-1.0.0.md](docs/notes-de-version-1.0.0.md).
