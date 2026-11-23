# Aidros `1.1.2` — les panneaux latéraux s'éveillent, la barre d'onglets ne se réveille plus elle-même

Un cran de **corrections**, après `1.1.0` et `1.1.1`. Deux chaînes cassées, toutes deux
**latentes et invisibles** jusqu'à ce que l'interface change d'état : les panneaux de la
barre latérale qui ne se chargent plus, et la barre d'onglets qui se redéclenchait
elle-même. Chaque correction a un filet qui mord, et le filet de la réserve de hauteur
avait **le même trou que le défaut** — il est réparé aussi.

**Projet dérivé, usage personnel.** Construit sur [**nathaninline/ajean**](https://github.com/nathaninline/ajean)
(projet principal, MIT), dont il descend en `0.20.1`. Les changements décrits ici forment
une **ligne privée** : non validés sur d'autres machines, non garantis transposables. Pour
installer AJEAN, prenez les versions publiées sur
[nathaninline/ajean](https://github.com/nathaninline/ajean/releases/latest).

## Les gains, chiffrés

| | avant | après |
|---|---|---|
| **Chargeurs lancés par `loadAll()`** | **zéro** (mort sur `ReferenceError`) | **dix-sept**, tous définis |
| **Panneaux latéraux peints** | **aucun des six** | **six**, prouvés à l'OCR sur la capture |
| **Écritures d'attributs par clic d'onglet** | une par minuterie, **à chaque passage** | **aucune** quand rien ne change |

## 1. `loadRemote` a survécu à son propre fichier — et `loadAll()` mourait avec lui

Symptôme vu sur le poste de fabrication : « **Configuration** » et « **Presets** », et
d'autres éléments de la barre latérale, **ne se chargent plus** — pendant que la RAM, elle,
est bonne. Deux choses distinctes, deux chemins distincts.

**La cause, mesurée dans le code :** le commit `f7e1318` (« Retirer Ajean Link du dépôt »,
22/11) a supprimé `13-remote.js` et sa fonction `loadRemote`, mais a **oublié de retirer
`loadRemote` de la liste** `const T=[…]` (`06-settings.js:722`). Citer un **identifiant
inexistant dans un littéral JS** lève `ReferenceError` **sur la ligne du tableau** :
`loadAll()` meurt **avant de lancer le moindre chargeur**. Les six blocs de `HRESERVE`
(`cfg`, `presets`, `internet-status`, `mcp-list`, `vram`, `ram-details`) gardent donc leur
hauteur réservée de la session précédente **et ne se peignent jamais**.

Le défaut dormait depuis la veille : le tableau est évalué à l'exécution du script, donc
**aucune requête réseau ne partait**, pas même la plus rapide. Il ne se voyait pas parce
que la page, elle, restait figée sur autre chose.

## 2. Le filet avait le même trou : compter n'est pas vérifier

`TestReserveDeHauteurLeveeParSonChargeur` **comptait** les dix-huit noms de la liste et ne
vérifiait la **définition** que des **propriétaires** de `HPROPRIETAIRE`. `loadRemote` n'en
est pas un : le compteur restait à 18, **le filet restait VERT avec une liste cassée**, un
jour entier.

Je répare donc **le filet aussi**, pas seulement le chiffre : le compte passe à dix-sept et
**chaque nom lancé doit être défini quelque part dans le JS**. Le filet a mordu dès le
premier essai, pour la bonne raison.

➜ **Leçon, à appliquer au-delà de ce fichier :** une liste dont je ne vérifie que la
**taille** peut être verte et cassée. Un littéral peut nommer un identifiant mort ; le
garde doit **résoudre chaque nom**, pas seulement les noms « importants ».

## 3. La barre d'onglets ne se réveille plus elle-même (`1.1.1`)

Le calque des onglets se rouvrait en boucle. **Ma première explication était fausse**, et
je l'ai corrigée en consultant la spécification DOM : `classList.toggle(token, force)`
exécute ses *update steps* **même quand le jeton ne change pas**, et « changer un
attribut » ne compare **jamais** l'ancienne valeur à la nouvelle. Autrement dit : écrire la
même valeur **est** une mutation, et elle **relance l'observateur**. D'où **trois gardes
explicites** — on ne touche à l'attribut que s'il doit changer :

```js
if (barre.hidden !== !visible) barre.hidden = !visible;
if (c.classList.contains('is-on') !== actif) c.classList.toggle('is-on', actif);
if (c.getAttribute('aria-current') !== courant) c.setAttribute('aria-current', courant);
```

plus les gardes sur `rel.hidden` / `pop.hidden`, et un `attributeFilter` restreint à
`['class','style']` sur l'observateur.

⚠ **Ce que ça ne prouve pas, dit franchement :** le gel n'est **pas** prouvé disparu. Je
n'ai jamais vu le calque se figer sous mes yeux, jamais vu non plus la preuve qu'il ne se
figerait plus. Ces gardes empêchent **l'écriture inutile** ; c'est la corde que l'observateur
pourchassait, pas un verdict sur le gel lui-même.

## Comment c'est prouvé

Relu **dans les octets de l'exe** (17 968 128 o, `41C8DD52…`, `FileVersion` =
`ProductVersion` = **1.1.2**) et **dans la page servie** (HTTP 200, 1 076 815 o) :

- **dix-sept chargeurs** dans la liste, **`loadRemote` x0** ;
- les six routes des panneaux répondent **200** (`/api/config`, `/api/presets`,
  `/api/internet`, `/api/mcp`, `/api/telemetry`) ;
- les gardes des onglets : `aria-current` **x1**, `classList.contains('is-on')` **x1**,
  `attributeFilter` **x5** (contre **x4** dans l'exe en service) ;
- **contre-épreuve** dans les deux exes `1.1.0` en service : `aria-current` **x0**,
  `attributeFilter` **x4**. Ce qui est neuf est bien **absent de l'ancien, présent dans le neuf** ;
- **l'OCR des deux captures** (`docs/ui-en.png`, `docs/ui-fr.png`, 2880×1800) lit
  `v1.1.2`, les deux onglets, **CONFIGURATION peuplée** (CTX 32 768, BATCH 2048,
  UBATCH 512, NGL 999) et les deux jauges (GPU 28,6 / 31,9 GiB, RAM 105,0 / 127,1 GiB).
  Une capture d'une page non rendue n'aurait pas ces chiffres ;
- `go vet ./...` **0**, filets de version **ok**, filets des onglets, de la marque, du
  badge et de la réserve **ok** ; `gofmt -l` vide.

Deux incidents de fabrication, nommés parce qu'ils sont instructifs : mon premier jet du
garde a **recopié le fichier de test** (12 498 → 200 890 caractères, mon bloc répété seize
fois) sans que je m'en aperçoive ; le contrôle de taille strict, ajouté du même coup, m'a
arrêté — et **mon attente était fausse elle aussi**, `18` → `17` étant deux chiffres pour
deux. ➜ pour une substitution, vérifier **le compte des deux côtés**, pas la longueur.

## Ce qui n'est pas là, dit franchement

- **Le gel de la barre d'onglets n'est pas prouvé parti** (voir le ⚠ plus haut).
- **Trois trous de traduction, vus par l'OCR de la capture anglaise** : le titre du pont
  reste « Pont Aidros ↔ Aidros », son badge « à l'écoute », et le sélecteur de projet
  « Générale ». Écrits **en dur dans le gabarit** (`index.tmpl.html:2263`,
  `13b-bridge.js:34`), donc hors de la portée d'`applyI18n()`. Nommés, **pas corrigés** :
  une modification à la fois.
- **Les minuteries de l'interface ne sont pas touchées.** Quatre tournent sous la seconde
  (200, 250, 500 ×2, 800 ms), dont une qui fait un `sync()` **réseau** tous les 500 ms.
- **Le premier rendu du fil ne page pas** : un fil de 148 108 jetons est rejoué en entier,
  alors que `/api/chat/older` répond en 0,014 s. C'est le plus gros des gains restants.
- **Aucun filet ne lit les images du README**, ni la release publiée : le workflow peut
  rougir et la release ne jamais naître, seule l'API GitHub le sait.
- La génération d'images n'est toujours pas codée ; pas de coffre à secrets, pas de
  sauvegarde ; les correctifs de l'amont ne tomberont plus tout seuls (leur `main` est un
  historique **sans ancêtre commun** avec le nôtre).

## État sur le poste de fabrication

Le binaire `1.1.2` est **posé aux deux endroits attendus** (racine et `data\bin`,
17 968 128 o / `41C8DD52…` / identité Windows **1.1.2**), sha relu **identique** à la
source aux deux places. **L'instance en service tourne sur le 1.1.2**, avec ses points de
retour (`aidros-1.1.1.exe`, `aidros.avant-1.1.0.bak.exe`). Mes essais ont été joués sur une
instance **isolée**, port **18095**, home séparé : rien tué, aucun port de service touché.

L'étiquette `v1.1.2` n'est **pas encore posée** : la publication attend un accord explicite.
Rappel mesuré le 22/11 : une étiquette `+revN` est **invisible au comparateur de versions**
(`Canonical` efface le `+`), donc **une release publiée est un numéro qui monte**, pas un `+rev`.

## Ce que cette version épingle

Inchangé depuis `1.1.0` — le millésime de l'application bouge, celui des moteurs non :

| | version |
|---|---|
| **moteur MoE Strata** | **0.1.59** (paquet `strata-engine-0.1.59-…-hip-rx9070`) |
| **panneau de contrôle** | **0.1.54** |
| **source Strata vendue** | **0.2.8** |
| **encodeur d'embeddings** | `embeddinggemma-2-Q8_0.gguf`, **309 855 456 octets**, llama.cpp b11669 |

⚠ **Un millésime est épinglé PAR POSTE, pas une valeur unique** : la variante sur RX 6900 XT
(gfx1030) vise une ligne RDNA2 distincte.

## Ascendance

- Base amont : `nathaninline/ajean` **0.20.1** (MIT).
- Moteur MoE : [**Niko1221/Strata**](https://github.com/Niko1221/Strata) — **0.1.59** posé,
  **0.1.54** de contrôle, source **0.2.8**.
- Embeddings : [**google/embeddinggemma-2**](https://huggingface.co/google/embeddinggemma-2),
  servi par [**ggml-org/llama.cpp**](https://github.com/ggml-org/llama.cpp).
- Notes de la version précédente (`1.1.0`) :
  [docs/notes-de-version-1.1.0.md](docs/notes-de-version-1.1.0.md).
