# Aidros `1.2.0` — la mémoire garde ce qu'elle a déjà encodé

Un cran de **fonction** après `1.1.3` : le chiffre du milieu monte sur l'arbitrage de
l'humain, « vu que l'on est sur une modification profonde de embedding ». La route des
embeddings n'a pas été réparée, **elle a changé de contrat**.

## Les gains, chiffrés

| | avant | après |
|---|---|---|
| fragments rescapés d'une coupure | **0** (tout-ou-rien) | **jusqu'à 1444 sur 1445** |
| cache disque, tel que `/api/embeddings` le rend | « **aucun cache disque** », en permanence | le fichier, relu et annoncé |
| travail jeté à un arrêt d'encodeur | **~12 min** (1316/1445 repris depuis le fragment 0) | **~11 s** (borné à 24 fragments) |
| coût de la sauvegarde pendant la route | — | **~0,3 %** (~30 ms d'écriture pour 0,47 s d'encodage) |
| encodeur tué après 20 min sans usage | **oui** — et il a tué une instance **morte** | **non** (garde d'inactivité levée) |

## 1. Le cache disque était inatteignable par construction

L'écriture sur le disque ne vivait qu'**après** le juge de fraîcheur `indexFresh`, dans
la **seule sortie sans `return`** de la route d'indexation. Un dossier de mémoire bouge —
le ménage de 05 h édite les pages — donc ce `return` rendait le fichier **jamais écrit**,
et `/api/embeddings` répondait « aucun cache disque » **en permanence**, job fini compris.

L'index est désormais posé **pendant** la route (tous les 24 fragments) et à **chacune**
de ses sorties : délai, annulation, erreur d'encodage, panne d'encodeur, et **avant** le
verdict du juge.

**Un anneau partiel est sûr, et c'est la clé qui le prouve** : `memoryChunk.Key` est
`memoryDigest(embeddingProtocol · fichier · titre · TEXTE · mediaHash)` — un vecteur rangé
sous la clé `K` a été calculé sur le texte qui hache en `K`. Il peut être **incomplet**,
il ne peut pas être **faux**.

**Le drapeau `Complet` ferme le mensonge que cette route introduisait** : un anneau
partiel a des empreintes de pages complètes, donc `indexFresh` — qui ne compare **que**
des empreintes — l'aurait déclaré **frais** avec des vecteurs manquants, et la recherche
aurait rendu **moins** de résultats sans le dire. Un anneau complet est **re-publié en
RAM** (le gain est là) ; un anneau partiel est **fondu par clé** dans l'index existant,
jamais publié comme frais.

## 2. La garde d'inactivité levée, dans les deux voies

Contexte mesuré : l'indexation du 23/11 s'était arrêtée à **1316/1445** sur une erreur
réseau et **ne repartait pas**. L'encodeur avait été tué **pour inactivité** par le
veilleur du 14/11 — une instance **morte** ; le job a reçu une erreur réseau au lieu
d'une relance.

⚠ `0` n'est pas « jamais » tout seul : `time.Since(…) > 0` est **vrai** aussitôt. Les
**deux** comparateurs testent d'abord l'**armement** — sinon la garde levée tuerait plus
vite que la garde armée. La garde RAM (700 Mo libres) et la tolérance de démarrage
(45 min, le temps d'installer `torch`) restent.

## Preuves

La morsure est **jouée dans les deux sens** : l'appel du tronçon critique **retiré** fait
**rougir** le garde, **remis** le fait **verdir**. Le garde est borné au **tronçon entre
le juge et son abandon** — chercher la sauvegarde dans tout le corps s'en satisferait
n'importe où, **y compris après**, ce qui est le défaut. Cinq filets neufs sur l'anneau
et la garde, dont deux qui rejouent la garde **armée** à une valeur écrite : deux de mes
filets jouaient une garde **levée** sans le savoir et verdiraient en ne prouvant rien.

`go vet` 0 · `gofmt` propre · **suite complète verte en 150,962 s** · quatre gardes de
version rejoués **non calculés** (`-count=1`).

Binaire de la campagne, cible portable `cmd/ajean-memory-test` : **17 981 952 octets**,
SHA-256 `7F632678…`, relu dans ses octets — UTF-16 (la ressource Windows) : « 1.2.0 » ×2,
« 1.1.4 » ×0, « 1.1.3 » ×0, « 1.1.0 » ×0. ASCII (les constantes Go) : non discriminant,
dit comme tel.
Les quatre `.syso` régénérés au **chemin complet** de `goversioninfo` — il n'est **pas au
`PATH`**, et `go generate ./...` **saute ce maillon sans se plaindre**.

## Ce qui n'est pas là

- **`aidros update` n'apporte rien sur ce poste** : `parallel_profile.go:282-284`
  court-circuite la route sans contacter GitHub. La release vaut pour une machine qui
  installe depuis GitHub.
- **Le job ne se relance pas tout seul.** Il garde son travail — mesuré : 10,1 Mo
  conservés, reprise en 363 s sans réécrire un vecteur — mais la reprise *automatique*,
  elle, n'est pas codée.
- **Les 30 `setInterval` de l'interface ne sont pas touchés** — c'est la cause mesurée de
  la RAM de Firefox (dont un `sync()` réseau tous les 500 ms). Ce cran ferme
  l'indexation, pas l'interface.
- **La garde RAM est hors d'atteinte ici** : seuil 700 Mo libres, mesuré **44 920 Mo**.
- La génération d'**images** n'est toujours pas codée (ComfyUI : **zéro occurrence** dans
  le dépôt). Pas de **coffre à secrets**, pas de **sauvegarde** (base bbolt à verrou
  exclusif, clé d'API en clair dans un profil). Les correctifs de l'amont ne tomberont
  plus tout seuls : depuis la scission, leur `main` est **sans ancêtre commun**.

Deux affirmations des notes de `1.1.3` deviennent fausses : « l'identité Windows du
binaire publié est la base nue `1.1.0` » (les quatre endroits sont montés, `1.2.0` relue
dans l'exécutable) et « **rien ne lit la release publiée** » (`TestPublicationNeSeProposePasEnBoucle`
le fait, en jouant le cas historique `Compare(v1.1.3, v1.1) = +1`).

## État sur le poste de fabrication

**La bascule est faite.** `aidros.exe` à la racine, 17 981 952 octets, `7F632678…`,
`ProductVersion`/`FileVersion` **1.2.0** relus dans le fichier, en écoute sur 18090 et
18190 depuis 22:01. Le point de retour de l'ancien (`aidros-1.1.3.exe`, `D657A229…`) reste
à la racine.

**L'épreuve de vie du correctif, mesurée sur le poste qui tourne** — c'est le seul endroit
où elle pouvait être jouée, les filets ne prouvaient que le code :

| ce que la campagne promet | lu sur le poste en service |
|---|---|
| le cache disque est **atteignable** | `cache : RAM + disque (.memcache/embeddings.json, 10 111 695 octets)` — là où il répondait « RAM ; aucun cache disque » **en permanence** |
| l'anneau se pose **pendant** la route | le fichier **grossit pendant** le job : 6 396 822 → 6 411 094 octets, `mtime` qui suit `job.Done` |
| la complétude **voyage avec le fichier** | `"anneau":"index complet"` relu dans la clé JSON du fichier |
| un arrêt en chemin **ne jette plus** le travail | le job s'est arrêté sur « fichiers modifiés pendant l'indexation » avec `Done 1579/1579` et **10,1 Mo conservés** ; relancé, il repart de zéro **sans réécrire les vecteurs** — le fichier ne bouge plus d'un octet, les clés se retrouvent |

**Le coût de la reprise, mesuré à deux reprises au lieu d'être supposé.** Le premier tour
s'est arrêté à 22:06:29, le second à 22:12:32 : **363 s pour 1579 fragments**, soit
**261 fragments/min** — contre **59 pages/min** au premier encodage, la mesure du 23/11 qui
faisait payer ~25 minutes. **La reprise est ~4,4 fois plus vite, et elle ne recode rien.**
C'est le gain promis, lu sur le poste qui tourne, pas dans un filet.

⚠ **Ce que le juge fait encore, et c'est voulu** : sur un dossier de mémoire qui bouge
**pendant** l'encodage — et il bouge, c'est justement le ménage de 05 h qui édite les pages —
`indexFresh` refuse de **publier** l'index en RAM et rend « fichiers modifiés pendant
l'indexation ; relancer ». L'anneau, lui, est sauvé. Donc la recherche retombe sur le repli
lexical tant que le dossier n'est pas calme, et il faut relancer le job : mesuré, la recherche
sur ce poste rend `resultats=1` en sémantique contre `repli lexical=5`. **Le défaut n'est pas
la campagne, c'est le couple** « le ménage édite la mémoire » × « l'indexation juge la mémoire
par son empreinte » : nommer le couple vaut mieux que bricoler l'un des deux.
**La première indexation complète reste donc due sur un poste qui écrit sa mémoire pendant
qu'il l'encode** ; elle ne se repaie plus au redémarrage, et se repaie quand **l'empreinte du
moteur** change — des vecteurs d'un autre moteur ne sont pas les miens.

## Ce que cette version épingle

Moteur MoE **Strata 0.1.59** (panneau de contrôle 0.1.54) · encodeur d'embeddings
**EmbeddingGemma-2** — **768 dimensions natives**, et le sélecteur de l'interface en
réduit l'empreinte à 512, 256 ou 128 (`mem_embedding_backend.go:235-241`, défaut 768 dans
`mem_embedding_settings.go:19`) : **512 sur ce poste**, réglage et non caractéristique de la
version · llama.cpp à la version épinglée par le poseur. La version de moteur est **par
machine**, et **avertit au lieu de refuser**. Cette campagne ne touche **aucun moteur** : les
épinglages sont reportés tels quels.

## Ascendance

- Base amont : `nathaninline/ajean` **0.20.1** (MIT).
- Moteur MoE : [**Niko1221/Strata**](https://github.com/Niko1221/Strata).
- Embeddings : [**google/embeddinggemma-2**](https://huggingface.co/google/embeddinggemma-2).
- Notes du cran précédent (`1.1.3`) :
  [docs/notes-de-version-1.1.3.md](docs/notes-de-version-1.1.3.md).
