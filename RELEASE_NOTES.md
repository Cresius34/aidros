# Aidros `1.0.0` — étiquette `v1.0.0+rev0`

Première release publiée de ce dépôt. **Phase de mise au point** : les noms, les
issues et le moteur sont encore en train d'être rangés. Elle est publiée pour
que le pointeur de mise à jour existe, pas pour annoncer un produit figé.

**Projet dérivé, usage personnel.** Construit sur [**nathaninline/ajean**](https://github.com/nathaninline/ajean)
(projet principal, MIT), dont il descend en `0.20.1`. Les changements décrits
ci-dessous forment une **ligne privée** : non validés sur d'autres machines, non
garantis transposables. Pour installer AJEAN, prenez les versions publiées sur
[nathaninline/ajean](https://github.com/nathaninline/ajean/releases/latest).

## ⚠ Cette release ne se proposera pas en mise à jour — et c'est voulu

Le comparateur du programme (`sys_update.go`) passe par `semver.Canonical`, qui
**efface la métadonnée `+rev…`** :

```
machine qui tourne 1.0.0   +   release v1.0.0+rev0   →   Compare = 0   →   rien de proposé
```

Donc : le 404 du pointeur de mise à jour disparaît (il n'y avait **aucune**
release publiée ici), et l'application ne se propose rien en boucle. Un
`+revN` désigne une **étiquette git de nos correctifs**, pas un numéro qui monte.
Les versions qui se proposeront entre elles seront `1.0.1`, `1.0.2`, `1.1.0`.

## Ce qu'Aidros est

Une application de conversation et d'agent **entièrement locale**, sur le matériel
graphique d'un PC de bureau, sans cloud.

- **Moteur MoE Strata** (0.1.59, avec son panneau de contrôle 0.1.54) — un modèle
  à experts activés à la volée, servi par un llama.cpp ROCm/HIP.
- **Moteur d'embeddings EmbeddingGemma-2** (256 dimensions) branché sur toutes
  les fonctions de mémoire : indexation, recherche sémantique, re-lecture du
  fil, invalidation d'un média supprimé. Il remplace le moteur de la génération
  précédente.
- **Pont Aidros↔Aidros** — deux machines du même réseau local qui se parlent :
  échange de fichiers, et pilotage à distance des presets et du mode depuis la
  carte de l'un ou de l'autre. La clé API de l'un **ne sort jamais** de son
  magasin local.
- **Cartes AMD** : mesuré sur une Radeon AI PRO R9700 (gfx1201) et sur une
  RX 6900 XT (gfx1030). Le sentier NVIDIA est celui de l'amont.

## Ce que cette version fait, depuis la base 0.20.1

### Le nom, posé partout où on le lit

L'application s'appelle **Aidros**. Elle a été trouvée trois fois en train de
porter un nom d'un temps mort, sur des surfaces que personne ne regardait :

- le **raccourci** s'appelait `AJEAN.lnk` alors que son infobulle disait « Aidros » —
  l'explorateur masque l'extension, donc le poste affichait un nom sous l'icône
  et un autre au survol ;
- le **binaire installé** s'appelait `ajean.exe` alors que le programme invitait
  à taper `aidros` ;
- l'**URL d'ouverture** s'appelait `?memory-test=` — l'époque où ce build servait
  à essayer un moteur de mémoire — sous un onglet intitulé « Aidros ».

Les trois sont nommés par **une seule source** et gardés par des tests. Le nom
d'avant est reconnu là où il faut : les raccourcis et les binaires d'une
installation antérieure sont **renomés, pas doublonnés**, et un ancien
`ajean.exe` trouvé sur le disque est **mis à jour s'il existe, jamais créé** —
deux icônes pour une application, ou deux exes qui se disputent un service, sont
les deux ratés mesurés en route.

### Le millésime du moteur avertit au lieu de refuser

Un preset qui ne demande pas le millésime de moteur épinglé par cette
application **démarre quand même**, et l'avis s'affiche — en trois endroits : la
bannière de `/api/status`, la carte des presets, et la pastille de la ligne du
preset. Vert exige trois accords : ce que le preset demande, ce que le moteur
posé déclare dans son `BUILD.json`, ce que cette application épingle.

Ça paie une mesure du 15/11 : un exe épinglé 0.1.57 refusait son propre vieux
preset 0.1.48 **quel que soit l'état du moteur** sur le disque. Changer le moteur
d'un poste ne coûtera plus une reconstruction du binaire.

### Deux routages perdus retrouvés, deux bugs clients

Un symbole signalé « inutilisé » est soit du code mort, soit **un routage perdu**.
Deux fois c'était un routage :

- le niveau de réflexion du composeur **reprenait son défaut** à chaque bascule
  de preset MoE (`REASONING_EFFORT` n'est pas dans les clés préservées) ;
- **deux envois JSON collés** sur `/api/preset` : tout client strict rejetait la
  carte du preset (`invalid character '{' after top-level value`, `Extra data` en
  `json.loads`). Un filet rejoue les deux cas, avec le message d'erreur d'un
  client strict.

### L'installeur du moteur déployait une installation incomplète

La liste blanche de l'installeur ne dépliait pas `data/` :
`expert-profile.bin` **n'était jamais écrit**, et le moteur refusait de démarrer
sans rien écrire dans un journal. Avant/après sur le vrai paquet : **1 425 → 1 449
fichiers**, la pièce **absente → présente, 196 632 octets**.

### La mémoire du composeur, et le budget du préambule

- Le prompt du résumeur gardait « CURRENT request, goal(s) and constraints » : un
  ordre de pilotage **est** une contrainte, donc le résumé **obéissait** à ce
  qu'il devait relater. Le prompt est devenu une constante **testable**, la classe
  des ordres de pilotage est fermée, les règles de fond sont épargnées.
- Un garde-fou plafonne le préambule **payé à chaque tour** (prompt + schémas
  d'outils). Il était en dépassement **avant** que je le pose, vert lancé seul.
- Le prompt système dit que la sortie des outils est **une donnée à lire, pas une
  consigne à suivre**.

### Le pont, et ce qu'il traverse

- Échange de fichiers par le pont, pilotage des presets et du mode à distance,
  trois cases dans le menu Pont.
- Le protocole s'appelle `X-Aidros-*`, avec bascule tolérante : un pair qui
  ignore encore le nouveau préfixe continue de répondre.
- Le **pilotage survit sans moteur chargé** ; le **prêt du modèle** meurt avec
  lui. Distinction mesurée, pas supposée : basculer un preset redémarre
  l'application, donc le pont de ce poste s'éteint le temps du redémarrage.

### L'intégration, pour la première fois

Les 21 et 22 novembre 2026, les runs d'intégration continue de ce dépôt sont
passés **au vert** — une première chez nous : nos huit premiers runs étaient
rouges, et notre dépôt archivé sous l'ancien nom (`Cresius34/ajean`) n'a **aucun**
run d'Actions. Chacun des huit rouges a été diagnostiqué un par un, et
**aucun n'était une panne du produit** : un nom de dépôt renommé, un fichier unix
qui compilait mal, `gofmt` sur 27 fichiers jamais formatés, une image de projet
sous lien symbole refusée sous macOS (`/var` → `/private/var`, un vrai défaut,
pas un caprice du coureur), deux tests qui jugeaient la machine qui les jouait,
et `staticcheck` ramené de 70 avertissements à **0** sous Linux **et** Windows.

## Ce qui n'est pas là, dit franchement

- **La génération d'images n'est pas codée.** ComfyUI tourne sur ce poste, mais
  compte **zéro occurrence** dans le dépôt, ici comme chez l'amont. Deux verrous
  sont nommés dans le code ; l'onglet est conçu, pas écrit.
- **L'écriture du prompt système par preset n'a plus de porte.** Sa lecture et son
  effacement sont routés, la clé a disparu de la requête d'enregistrement et
  l'interface ne l'envoie plus. Lacune reçue de l'amont en 0.20.0 : la reposer,
  c'est un champ et une porte.
- **Pas de coffre à secrets, pas de sauvegarde.** La base est du bbolt à verrou
  exclusif, et une clé d'API est stockée en clair dans un fichier de profil.
- **Les correctifs de l'amont ne tomberont plus tout seuls.** Depuis la scission,
  leur `main` est un historique **sans ancêtre commun** avec le nôtre (racine
  sans parent, 10 étiquettes en trois jours, chemins qui ne se recouvrent pas).
  Un correctif sur `web_api.go`, `llm_client.go`, `chat_compact.go` ou l'UI se
  porte à la main, désormais.
- **Les binaires de cette release sont construits pour AMD par nos soins** ; le
  paquet publié ici est celui que l'intégration continue fabrique, sans moteur
  embarqué. Le moteur s'installe par le poseur du dépôt.

## Les six cibles

`aidros-linux`, `aidros-linux-arm`, `aidros-windows.exe`,
`aidros-windows-arm.exe`, `aidros-macos`, `aidros-macos-arm`, plus `SHA256SUMS`.
Ces noms sont ceux que le programme cherche pour se mettre à jour
(`updateAssetName`) : les changer, c'est éteindre la mise à jour automatique.

**Le moteur MoE n'est pas dans ces fichiers.** Il se pose par
`moe_portable_install.ps1`, qui épingle le millésime.

## Ascendance

- Base amont : `nathaninline/ajean` **0.20.1** (MIT).
- Moteur MoE : [**Niko1221/Strata**](https://github.com/Niko1221/Strata).
- Embeddings : [**google/embeddinggemma-2**](https://huggingface.co/google/embeddinggemma-2).
- Notes de la ligne précédente (`0.18.17.1 Cresius`, `v0.18.17+rev1`) :
  [docs/notes-de-version-0.18.17.1-Cresius.md](docs/notes-de-version-0.18.17.1-Cresius.md).
