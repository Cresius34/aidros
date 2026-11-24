# Aidros `1.1.3` — la carte du Pont parle la langue de l'écran

Un cran de **corrections** après `1.1.0`, `1.1.1` et `1.1.2`. Le défaut était visible
sur une capture d'écran et invisible dans le code : l'interface servie en **anglais**
gardait « Pont Aidros ↔ Aidros », son badge « à l'écoute » et le sélecteur de projet
« Générale ». Trois trous, nommés dans mes notes de livraison du 23/11 et laissés là
faute de correctif. La cause n'était pas une clé mal traduite : **ces textes
n'avaient aucune clé.**

**Projet dérivé, usage personnel.** Construit sur [**nathaninline/ajean**](https://github.com/nathaninline/ajean)
(projet principal, MIT), dont il descend en `0.20.1`. Les changements décrits ici forment
une **ligne privée** : non validés sur d'autres machines, non garantis transposables. Pour
installer AJEAN, prenez les versions publiées sur
[nathaninline/ajean](https://github.com/nathaninline/ajean/releases/latest).

## Les gains, chiffrés

| | avant | après |
|---|---|---|
| textes de la carte du Pont portant une clé | **0** | **23** (contrôlés un à un dans le gabarit) |
| lignes de français en dur dans `13b-bridge.js` | **33** | **0** |
| clés de traduction, français / anglais | 0 / 0 | **74 / 74**, parité vérifiée |
| écrans anglais affichant du français | 3 | **0** (prouvé par la page rendue) |

## 1. `applyI18n()` ne traduit que ce qui porte une clé — et la carte n'en portait pas

Le moteur parcourt les éléments marqués `data-i18n`. La carte Pont était écrite en
français **en dur**, dans le gabarit comme dans son JS : `applyI18n()` passait devant
sans rien trouver à traduire. D'où le piège du 23/11 : un texte traduit à 100 % dans le
dictionnaire, mais **absent du dictionnaire**, se traduit parfaitement en… rien.

74 clés posées dans les deux langues : **73 `bridge.*` + `projects.default_name`**. Le
français de secours reste écrit dans le gabarit à côté de sa clé : c'est le repli prévu
du moteur, pas une trace du défaut.

## 2. Le nom du projet par défaut demandait plus qu'un mot à traduire

Le serveur fige `defaultProjectName = "Générale"`, et **c'est ce nom qui nomme le
dossier mémoire** `memory/<slug>/`. Le nom *stocké* ne bouge donc pas ; ce qui se dit
dans la langue est le **libellé**.

Deux drapeaux, vrais seulement si le projet porte **encore** le nom figé :

- `name_default` sur `/api/projects`,
- `default_name_default` sur `/api/chat/projects`.

Sans ce drapeau, traduire écraserait un nom que l'humain a écrit : le « Projet » d'un
projet rebaptisé « Mon atelier » reste « Mon atelier » dans les deux langues.

## 3. Changer de langue ne repaie pas l'ouverture d'un tchat

`setLang()` repeint le nom **sans appel réseau** : les projets sont déjà en mémoire.
Je n'appelle **ni** `loadProjects()` **ni** `loadHistory()` :

- le nombre de chargeurs de `loadAll()` est un **contrat gardé par un test**
  (`ui_reserve_hauteur_test.go` : « 17 chargeurs ») ;
- `loadHistory()` reconstruit tout le fil — **mesuré 148 108 jetons** le 23/11. Payer
  ça pour un seul mot rendrait un changement de langue **plus lent qu'une ouverture de
  tchat**.

## Comment c'est prouvé

`i18n_pont_test.go`, **six filets**, joués **dans les deux sens** :

| contre le code **non traduit** (`git stash`) | contre le code traduit |
|---|---|
| **23** textes visibles sans `data-i18n` | six **verts** |
| **33** lignes de français en dur | — |
| **0** clé surveillée | **74** clés, parité fr/en |

Le filet a trouvé **un vrai trou que j'avais laissé** : `'refusé'` en dur ligne 174 —
j'avais traduit les autres fonctions de la carte et oublié celle-là. Et un **faux
positif où le filet avait tort** : `const moteur = pre.moteur ? …` accusé d'être du
français. Ces mots servent d'identifiants JS dans ce fichier ; les retirer de la liste
est le correctif, pas un arrosage — l'accentué les couvre dès qu'ils sont accentués.

**La preuve ne vient pas que des octets.** Sur un atelier isolé, moteur décalé, après
`POST /api/prefs {"lang":"en"}`, la page servie rend `Receive a brain` ×1, `Listening`
×3, et `à l'écoute` **×0**. Les octets, dans les deux sens : le neuf porte
`data-i18n="bridge.title"` ×1, `name_default` ×4, `default_name_default` ×2,
`projects.default_name` ×4 ; le 1.1.2 en service les rend à **×0** et porte encore
`stitle">Pont Aidros` ×1, qui a disparu du neuf.

## Ce qui n'est pas là, dit franchement

- **Une ligne d'historique déjà affichée garde son étiquette de projet** jusqu'au
  prochain rechargement de l'historique. C'est une ligne de liste, pas l'écran courant ;
  le payer à chaque changement de langue coûtait la reconstruction du fil.
- **La garde du texte en dur est lexicalisée, pas grammaticale.** Elle surveille les
  accents, les apostrophes typographiques, et une liste fermée des mots d'écran de
  **cette** carte. Un mot français sans accent et sans majuscule (« prêt », « ici »)
  écrit en dur ailleurs passerait. L'étendre à toute l'interface se paierait en faux
  positifs sur le français de secours : l'élargir est une décision, pas un réglage.
- **La translation est bornée à la carte du Pont**, là où le défaut a été mesuré. Le
  reste de l'interface n'est pas couvert par ces six filets.
- **Le binaire publié par l'intégration continue est `./cmd/aidros`**, dont l'identité
  Windows est la **base nue** (`1.1.0`) : le cran de correctif `1.1.3` vit dans le build
  portable, seul à poser `BuildFlavor`. C'est mesuré, pas supposé.
- `goversioninfo` **n'est pas au `PATH`** sur ce poste : `go generate ./...` le saute et
  **ne régénère pas les `.syso`**. Les `.syso` de la cible portable ont été régénérés à
  la main, en chemin complet, et relus (« 1.1.3 » ×2, « Aidros » ×4 chacun).
- **Rien ne lit la release publiée.** Aucun filet ne vérifie que l'étiquette publiée
  est en phase avec le millésime du binaire qu'elle embarque.
- **La génération d'images n'est toujours pas codée** : ComfyUI tourne sur ce poste, mais
  compte **zéro occurrence** dans le dépôt, ici comme chez l'amont.
- **Pas de coffre à secrets, pas de sauvegarde.** La base est du bbolt à verrou exclusif,
  et une clé d'API est stockée en clair dans un fichier de profil.
- **Les correctifs de l'amont ne tomberont plus tout seuls.** Depuis la scission, leur
  `main` est un historique **sans ancêtre commun** avec le nôtre.

## État sur le poste de fabrication

`aidros-1.1.3.exe` en service, **17 980 416 octets**, SHA-256 `D657A229…`,
`ProductVersion 1.1.3`, `ProductName Aidros`. L'interface rend `/api/status → version
1.1.3, state active`, moteur MoE en vie. `go vet` 0, `gofmt` propre, **suite complète
verte en 137,9 s**, les trois gardes de version verts dans les deux sens.

## Ce que cette version épingle

Moteur MoE **Strata 0.1.59** (panneau de contrôle 0.1.54) · encodeur d'embeddings
**EmbeddingGemma-2**, 256 dimensions · llama.cpp à la version épinglée par le poseur.
La version de moteur est **par machine**, et **avertit au lieu de refuser**.

## Ascendance

- Base amont : `nathaninline/ajean` **0.20.1** (MIT).
- Moteur MoE : [**Niko1221/Strata**](https://github.com/Niko1221/Strata).
- Embeddings : [**google/embeddinggemma-2**](https://huggingface.co/google/embeddinggemma-2).
- Notes du cran précédent (`1.1.2`) :
  [docs/notes-de-version-1.1.2.md](docs/notes-de-version-1.1.2.md).
