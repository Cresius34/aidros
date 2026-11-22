# Notices

## Ascendance

Ce dépôt est un **fork** de
[**nathaninline/ajean**](https://github.com/nathaninline/ajean), le projet
d'origine — la source des versions publiées sous ce nom-là. Le nom `ajean` de
l'amont est cité ici **en tant que nom de l'amont**, pas en tant que nom de ce
produit : depuis le 21/11, **ce produit s'appelle Aidros**.

Le 20/11, l'amont a réécrit son historique et publié une lignée `v1.0.x` sans
ancêtre commun avec la nôtre (mesuré : `git merge-base` ne rend **rien**). Nos
modifications vivent donc sur notre propre sol, `aidros`, et l'amont a annoncé
qu'il **migrait nos données** depuis l'ancien AJEAN 0.x. Cette page décrit le
nôtre.

## Ce que ce fork ajoute à l'amont

Sept fronts, tous dans notre code, aucun dans le leur :

| Front | Ce qui est ajouté |
| --- | --- |
| **Moteur MoE Strata** | `backend_moe_strata.go`, `backend_moe_windows/_linux/_other.go`, `moe_probe.go` (route `/api/moe`), catalogue de modèles **lu dans le moteur** plutôt que recopié à la main |
| **Mémoire sémantique EmbeddingGemma 2** | `mem_embeddings.go`, `mem_embedding_backend/_media/_settings/_transformers.go`, `embedding_server.py` — un moteur d'embeddings local, sur carte AMD |
| **Pont Aidros↔Aidros** | `parallel_bridge.go`, `parallel_peer.go`, `parallel_bridge_agent/_control/_files/_pilot.go` — deux instances d'un même réseau local se parlent, chacune garde sa mémoire ; protocole `X-Aidros-*` |
| **Déduplication des appels d'outils** | `repeat_exempt.go`, tests `llm_dedup_*` |
| **Outillage d'installation Windows** | `tools/patch-canal/` — poseur, points de retour, pose du moteur sur carte AMD |
| **Réglage du seuil de compactage** | `COMPACT_FRAC` (pourcentage) + marge de sécurité |
| **Ascendance assumée** | ce fichier, entête `LICENSE`, README FR/EN, `RELEASE_NOTES.md` |

## Ce qui est embarqué, et ce qui ne l'est pas

Écrit parce que la différence était régulièrement floue. Mesuré dans le dépôt le
21/11, et non d'après les intentions :

- **Embarqué** : moteur MoE Strata, moteur d'embeddings local, pont entre
  instances, pilotage à distance du voisin (presets, mode), mémoire par projet,
  trackers, applications, tâches planifiées, MCP, point d'accès compatible
  OpenAI.
- **Retiré de l'interface, gardé comme matière première** : la carte **AJEAN
  LINK** (masquée le 21/11). Le relais `ajean.link` appartient à l'opérateur de
  l'amont et garde son nom ; il n'est pas à nous.
- **Absent du dépôt** : **ComfyUI** — mesuré à **0 occurrence dans le code**
  (`.go`, `.js`, `.html`, `.ps1`, `.py`, `.json`), aucun dossier, aucun script.
  La génération d'images est **une fonction envisagée, pas une fonction
  livrée**. Le dépôt supprime explicitement le générateur d'images de la
  configuration du moteur (`delete(out, "images")` dans
  `backend_moe_strata.go:292`). Un prototype mesuré, hors dépôt, existe.
- **Absent aussi** : `/api/secrets` et `/api/images` (mesuré à 0 occurrence) —
  un coffre à secrets et une sauvegarde sont les deux trous nommés à combler.

## Ce que Windows et macOS affichent de nous

Le nom, plus un changelog. La règle est posée et gardée par un test
(`logo_marque_test.go`, qui lit le fichier **généré** `ui/index.html`, pas sa
source) : ce que l'explorateur Windows, la barre de menus macOS et le raccourci
affichent **portent un nom** — `Aidros`. Le détail chiffré est dans
[RELEASE_NOTES.md](RELEASE_NOTES.md).

## Deux millésimes, deux sens

Un **preset** nomme le millésime de moteur qu'il **demande** ; le **moteur posé
à côté de l'exécutable** déclare celui qui **est là** (dans son `engine/BUILD.json`).
Depuis le 21/11, un désaccord **avertit et s'affiche** (pastille verte ou rouge
sur la ligne du preset, avis nommé dans l'interface) au lieu de **refuser** le
démarrage : un preset rouge **démarre quand même**. Un seul exécutable peut donc
servir plusieurs millésimes de moteur — changer le moteur d'une machine voisine
ne coûte plus de recompilation. L'installeur, lui, épingle toujours.

## Modèles

Les modèles téléchargés par le programme sont distribués sous leurs propres
licences (voir la page du modèle concerné sur Hugging Face) ; leur utilisation
est régie par les conditions du modèle, pas par la licence d'Aidros.

## Dépendances

Aidros s'appuie sur les composants suivants, chacun sous sa licence :

- **llama.cpp** (MIT) — moteur d'inférence, utilisé via les binaires officiels
  publiés sur les releases `ggml-org/llama.cpp`.
- **Strata** (moteur MoE, voir la licence du dépôt `Niko1221/Strata`) — pack
  Windows téléchargé à l'installation du moteur MoE.
- **EmbeddingGemma 2** (`google/embeddinggemma-2`) — modèle d'embeddings de la
  mémoire sémantique, sous sa propre licence.
- **bbolt** (Apache-2.0) — base locale `ajean.db`. Ce fichier **garde son nom** :
  c'est le nom sous lequel il est écrit sur les machines déjà installées.

## Le nom, et ce qui ne se renomme pas

Le produit, l'exécutable et le paquet Go s'appellent **Aidros** (mesuré : les
fichiers de release sont `aidros-linux`, `aidros-windows.exe`, etc. ; le paquet
est `internal/aidros`). Trois choses gardent `ajean`, **parce que le code les
lit sous ce nom** et que les renommer dans un document serait un mensonge :

- les **variables d'environnement** : `AJEAN_HOME`, `AJEAN_SERVICE`,
  `AJEAN_MODEL_DIRS`, `AJEAN_CHROME`, `AJEAN_DL_CONNS` (mesuré : `AIDROS_HOME`
  compte **0** occurrence dans le code) ;
- les **services systemd** `ajean-engine` / `ajean-ui` et le fichier
  `/etc/ajean` (mesuré : `aidros-engine` compte **0** occurrence) ;
- la **base** `ajean.db`.

Le dossier de données s'écrit **`aidros`** à l'installation, et **`ajean` reste
admis** en second — une machine installée avant le renommage y a déjà sa mémoire.
