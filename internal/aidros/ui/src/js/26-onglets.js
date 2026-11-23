// 26-onglets.js — la barre d'onglets : Conversation / Mémoire sémantique.
//
// L'ordre de l'humain (23/11) : « une barre horizontale qui fasse office d'onglet
// pour basculer d'une interface à l'autre SANS OUVRIR DE NOUVELLE PAGE du
// navigateur ». Le lien de la mémoire sémantique portait `target="_blank"` : chaque
// clic ouvrait un onglet de navigateur, et au deuxième on se retrouvait avec deux
// Aidros ouverts, l'un à jour de la clé d'accès, l'autre pas.
//
// ⚠️ CE QUE LE MOT « ONGLET » NE VEUT PAS DIRE ICI. Ces boutons ne basculent pas
// deux <div> qu'on montre et cache : la page de mémoire sémantique est une AUTRE
// PAGE, servie par une autre route (`/semantic-memory`, web_server.go:140 →
// handleEmbeddingPage), avec son propre <style> qui redéfinit body, h1 et button,
// et ses propres fonctions globales — `load`, `api`, `search`, `saveConfig`,
// `pickers` — des noms GLOBAUX, donc en concurrence avec ceux de cette app.
// ⚠ L'exemple que ce commentaire donnait ici (« 07b-memory.js a son `load` ») est mort
// le 23/11 au soir : le panneau de réglages mémoire de l'accueil est sorti au profit de
// l'onglet, et son fichier avec. Mesuré après coup : plus aucun JS de l'app ne définit
// `function load(`. La séparation par iframe reste la bonne raison — c'est la PAGE SERVIE
// qui porte ces globaux (relu dans son <script> : `api`, `load`, `search`, `saveConfig`,
// `pickers`) — mais un commentaire qui cite un fichier disparu ment à celui qui le lit.
// Les recopier dans cette
// page, c'était les faire se marcher dessus, et repeindre le fil avec le CSS de
// l'autre. L'iframe est ce qui garde la séparation SANS dupliquer le code : la page
// servie reste la seule copie, ici comme ailleurs.
//
// ⚠️ L'IFRAME NE SE DÉCIDE PAS TOUJOURS. Le voile de repli ci-dessous n'est pas un
// ornement : une iframe refuse de se peindre si la réponse nommée porte
// `X-Frame-Options` ou une CSP `frame-ancestors` qui l'interdit, et ce refus est
// SILENCIEUX — aucune erreur, aucun événement, un rectangle vide. Un calque vide a
// l'air d'un bug de l'app, pas d'une règle du serveur. Alors on relit les en-têtes
// AVANT de poser l'iframe, et ce qu'on apprend là décide du calque ou de la fenêtre.

var TAB_MEMORY_URL = '/semantic-memory';
var tabActif = 'chat';
var tabMemoirePret = false;   // l'iframe est posée et acceptée par le serveur
var tabMemoireInterdit = false; // le serveur refuse d'être encadré : on l'a mesuré

// setTabsVisible : la barre ne se montre que si elle a quelque chose à offrir.
//
// Le lien de la mémoire est dans #mem-block, un `gated-group` : le verrouillage de
// la mémoire le masque (voir setAgentGate, 06-settings.js:391 — et non 05-status.js,
// que ce commentaire nommait à tort ; mesuré : zéro occurrence de « gated » dans
// 05-status.js). Poser une barre avec un seul onglet, c'est offrir un bouton mort
// à l'humain. Donc : on la montre quand la mémoire existe, on la retire quand elle
// disparaît, et si l'onglet mémoire était ouvert au moment où elle s'éteint, on
// revient à la conversation — sinon on resterait sur un calque dont la porte vient
// d'être condamnée.
//
// ⚠️ LE LIEN SEUL NE SUFFIT PAS, et c'est le défaut que Julien a vu le 23/11 au
// soir : « tu vois une barre d'onglet là-dessus, en haut à l'horizontale ? » — il ne
// la voyait pas, et il ne POUVAIT pas la voir. `#mem-block` vit dans `#mem-modal`
// (index.tmpl.html:984, une `modal-ov` en `display:none` que seul openMemHub ouvre,
// 18-projects.js:206). Le lien n'existe donc dans le DOM QUE quand la modale
// Mémoire est ouverte. Une barre dont l'existence dépend d'une modale fermée est
// invisible jusqu'au jour où l'humain ouvre cette modale — et l'ordre qu'il a donné
// était de pouvoir basculer entre les sections. La barre doit être là AVANT.
//
// ⚠️ La source de vérité n'est donc plus le DOM, c'est le SERVEUR : `tabMemoireDispo`,
// lu dans `/api/embeddings` → `config.enabled` (mesuré : répond en 10 ms,
// `{"config":{"enabled":true,…}}`). Le lien reste écouté par l'observateur, donc si
// la mémoire se verrouille et que le lien part, `tabMemoireDispo` retombe et la
// barre aussi. Un `enabled: true` qui n'offre aucun chemin utilisable montrerait une
// barre vide : c'est le risque de ce geste, il est pris sciemment, l'API est ce que
// l'app a de mieux comme avis sur la mémoire.
var tabMemoireDispo = false;

function setTabsVisible() {
	var barre = document.getElementById('tabs');
	var lien = document.querySelector('#mem-block a[href="/semantic-memory"]');
	var visible = !!lien || tabMemoireDispo;
	if (!barre) return;
	if (barre.hidden !== !visible) barre.hidden = !visible;
	if (!visible && tabActif === 'memory') goTab('chat');
	updateTabButtons();
}

// goTab est le seul chemin qui change l'onglet affiché. Appelé par les deux
// boutons, par le lien de la modale Mémoire, et par setTabsVisible.
function goTab(onglet) {
	if (onglet === 'memory') {
		openMemoryTab();
	} else {
		closeTab();
	}
	updateTabButtons();
}

function updateTabButtons() {
	var chat = document.getElementById('tab-chat');
	var memoire = document.getElementById('tab-memory');
	var etre = function (el, actif) {
		if (!el) return;
		if (el.classList.contains('is-on') !== actif) el.classList.toggle('is-on', actif);
		var courant = actif ? 'page' : 'false';
		if (el.getAttribute('aria-current') !== courant) el.setAttribute('aria-current', courant);
	};
	etre(chat, tabActif === 'chat');
	etre(memoire, tabActif === 'memory');
	// Recharger et « ouvrir ailleurs » ne se conçoivent que sur un calque.
	var dansCalque = tabActif === 'memory';
	var rel = document.getElementById('tab-reload');
	var pop = document.getElementById('tab-pop');
	if (rel && rel.hidden !== !dansCalque) rel.hidden = !dansCalque;
	if (pop && pop.hidden !== !dansCalque) pop.hidden = !dansCalque;
}

// tabEnCadreRelu : ce que le serveur permet, lu dans ses en-têtes. Résolu UNE fois
// et gardé — c'est une question au serveur, pas un état qui change tout seul.
var tabEnCadreRelu = null;

async function tabEnCadrePermis() {
	if (tabEnCadreRelu !== null) return tabEnCadreRelu;
	var permis = true;
	try {
		var ctrl = new AbortController();
		var delai = setTimeout(function () { try { ctrl.abort(); } catch (e) {} }, 4000);
		var res = await fetch(TAB_MEMORY_URL, { method: 'HEAD', cache: 'no-store', signal: ctrl.signal });
		clearTimeout(delai);
		// ⚠️ Le CODE, pas seulement les en-tetes. Defaut trouve en montant
		// l'eprouvette : l'iframe y pointait /semantic-memory, que son petit
		// serveur ne connait pas — reponse 404, aucun X-Frame-Options, donc ma
		// regle disait « permis », l'iframe se chargeait sur la page d'erreur,
		// son onload se declenchait, et l'humain voyait un `404: Not Found` tout
		// nu dans un calque que l'app croyait READY. Un calque qui affiche une
		// erreur et que le code tient pour reussi est le pire des deux : le
		// repli, lui, aurait propose la fenetre du navigateur.
		//
		// 405 (et 501) EXCEPTES : c'est la METHODE qui plait pas au serveur, pas
		// la page. Refuser la, ce serait condamner l'onglet sur un serveur qui
		// sert tres bien la page en GET.
		if (res.status === 405 || res.status === 501) {
			// on ignore le statut, on ne regarde que les en-tetes
		} else if (res.status >= 400) {
			permis = false;
		}
		var xfo = (res.headers.get('x-frame-options') || '').trim().toUpperCase();
		// Règle FERMÉE, et dans le sens prudent : on n'accepte que SAMEORIGIN, et
		// on refuse tout le reste — DENY, ALLOW-FROM, et surtout une valeur qu'on
		// ne connaît pas. Ma première version cherchait « ALLOW-FORM » : j'avais
		// écrit FORM au lieu de FROM, et le garde passait sur la valeur réelle du
		// serveur en la croyant inconnue. C'est la sonde qui l'a trouvée, pas la
		// lecture. Enumérer des mots, c'est s'exposer à ce genre de faute ;
		// n'admettre qu'une seule valeur, non.
		if (xfo && xfo !== 'SAMEORIGIN') permis = false;
		// CSP : on ne cherche que `frame-ancestors`, et on ne l'interprète pas
		// clause par clause. Une CSP qui en nomme un est une intention de
		// l'auteur ; sans certitude lisible, on ne pose pas un calque vide.
		var csp = res.headers.get('content-security-policy') || '';
		if (permis && /frame-ancestors/i.test(csp)) {
			var clause = csp.split(/;\s*/).filter(function (c) { return /^frame-ancestors/i.test(c.trim()); })[0] || '';
			permis = /'self'|~\/|\bhttps?:\/\/|file:|\*/i.test(clause.replace(/frame-ancestors/i, ''));
		}
	} catch (e) {
		// Un échec de lecture n'est pas une interdiction : le HEAD peut échouer
		// (méthode non permise, réseau court) là où l'iframe irait très bien.
		// On tente, et le voile de repli reste sous l'iframe pour l'échec réel.
		permis = true;
	}
	tabEnCadreRelu = permis;
	return permis;
}

async function openMemoryTab() {
	tabActif = 'memory';
	var calque = document.getElementById('mem-tab');
	var repli = document.getElementById('mem-tab-off');
	if (calque) calque.hidden = false;
	if (repli) repli.hidden = true;
	if (tabMemoirePret) return;
	if (!(await tabEnCadrePermis())) {
		tabMemoireInterdit = true;
		tabMemoirePret = false;
		if (repli) repli.hidden = false;
		var cadre = document.getElementById('mem-frame');
		if (cadre) cadre.src = 'about:blank';
		return;
	}
	tabInstaller();
}

// tabInstaller pose l'iframe et sa charge. La charge ne se fait qu'ici, jamais à
// l'ouverture de la barre : la page de mémoire appelle /api/embeddings et relance
// un sondage de deux secondes (son `setInterval(load, 2000)`), plus les sélecteurs
// de chemins. La charger d'office, ce serait payer un encodeur et un aller-retour
// par ouverture d'Aidros, y compris pour l'humain qui ne regarde jamais la mémoire.
//
// ⚠️ Le DELAI de garde ci-dessous n'est pas un ornement de plus : le refus d'un
// `X-Frame-Options` est SILENCIEUX. Ni `onload`, ni `onerror`, ni exception — le
// cadre reste vide. J'ai bien lu les en-têtes avant de poser l'iframe, mais un
// en-tête peut n'être posé que sur la réponse de la page (le HEAD peut suivre une
// autre route, un autre middleware). Sans ce delai, un tel refus laisserait un
// rectangle vide sans issue ; avec, l'humain voit le bouton qui l'en sort.
// 8 s : la page de mémoire charge 1,11 Mo d'interface avant d'appeler son API, sur
// une machine chargée ce n'est pas instantané, et un faux repli serait pire que le
// vide qu'il remplace.
var tabGardeDelaiMs = 8000;
var tabGarde = null;

function tabInstaller() {
	var cadre = document.getElementById('mem-frame');
	var repli = document.getElementById('mem-tab-off');
	if (!cadre) return;
	cadre.onload = function () {
		tabMemoirePret = true;
		tabMemoireInterdit = false;
		if (tabGarde) { clearTimeout(tabGarde); tabGarde = null; }
		if (repli) repli.hidden = true;
	};
	cadre.onerror = function () {
		if (repli) repli.hidden = false;
	};
	if (tabGarde) clearTimeout(tabGarde);
	tabGarde = setTimeout(function () {
		tabGarde = null;
		if (!tabMemoirePret && repli) repli.hidden = false;
	}, tabGardeDelaiMs);
	cadre.src = TAB_MEMORY_URL;
}

// closeTab rend la main ET décharge l'iframe.
//
// ⚠️ `hidden = true` seul ne rend RIEN. Le calque est caché à l'œil, l'iframe est
// toujours dans le DOM : son document vit, ses promesses aussi, et surtout sa
// minuterie. Relue à l'instant, la page encadrée (`/semantic-memory`, 12 944 o)
// porte `setInterval(load, 2000)` et AUCUN garde `visibilitychange` — mesuré :
// 0 occurrence. Un `setInterval` dans l'iframe d'un onglet VISIBLE n'est pas
// bridé par le navigateur. Résultat : une seule ouverture de l'onglet = un
// sondeur de 2 secondes à perpétuité, fetch derrière fetch, jusqu'à la fermeture
// de l'onglet du navigateur. Sur une machine à 1-4 Go libres, chaque battement
// re-faute des pages : ça ne consomme pas des giga-octets, ça empêche de dormir.
// C'est le défaut que Julien a nommé d'abord, en demandant des onglets « sans
// ouvrir de nouvelle fenêtre ni d'instance en dehors ».
//
// ⚠️ `about:blank` est le geste, pas le retrait de l'élément. Naviguer l'iframe
// détruit son document, sa fenêtre globale et donc sa minuterie ; l'élément vide
// ne pèse rien. RETIRER l'élément aurait été un piège : `tabInstaller()` fait
// `getElementById('mem-frame')` et `return` s'il est absent — il aurait fallu le
// recréer, et un oubli ici ouvrait un calque vide. `reloadTab` (au bas de ce
// fichier) fait déjà exactement ce déchargement, pour la même raison.
//
// ⚠️ `tabMemoirePret` TOMBE À FAUX, il faut le remettre à faux ici. Sans ça,
// `openMemoryTab()` retombe sur son `if (tabMemoirePret) return` et rouvre un
// calque que le code tient pour prêt alors qu'il vient d'être vidé — le pire des
// deux. `tabEnCadreRelu`, lui, est GARDÉ : c'est une question au serveur, pas un
// état qui change tout seul (voir son commentaire en tête de fichier).
//
// ⚠️ Le `onload` posé par `tabInstaller` se déclenche AUSSI sur `about:blank` :
// il écrirait `tabMemoirePret = true` sur la décharge, et effacerait juste après
// la remise à faux du paragraphe ci-dessus. D'où les deux handlers coupés
// AVANT de changer la source.
function closeTab() {
	tabActif = 'chat';
	var calque = document.getElementById('mem-tab');
	if (calque) calque.hidden = true;
	tabMemoirePret = false;
	var cadre = document.getElementById('mem-frame');
	if (cadre) {
		cadre.onload = null;
		cadre.onerror = null;
		cadre.src = 'about:blank';
	}
}

// reloadTab rejoue la charge du calque. ⚠ On ne remet PAS la même valeur : poser
// `cadre.src = cadre.src` est tenu pour une source INCHANGÉE par le navigateur, qui
// ne recharge rien du tout. D'où about:blank d'abord, qui est bien un changement.
function reloadTab() {
	var cadre = document.getElementById('mem-frame');
	if (!cadre) return;
	tabMemoirePret = false;
	cadre.src = 'about:blank';
	setTimeout(function () { cadre.src = TAB_MEMORY_URL; }, 60);
}

// popTab : la sortie de secours, et le chemin d'origine.
function popTab() {
	window.open(TAB_MEMORY_URL, '_blank', 'noopener');
}

// Le lien de la modale Mémoire devient un bouton d'onglet.
//
// ⚠ On ne REMPLACE PAS le lien, on lui prend son clic : le HTML de la modale reste
// la source de la vérité sur « est-ce que la mémoire sémantique existe pour ce
// projet », et un seul endroit du gabarit en parle. Le garde logo_marque_test.go
// relit d'ailleurs cette page servie ; ne pas dupliquer le libellé, c'est ne pas
// avoir à le maintenir deux fois.
//
// ⚠️ `href` RESTE, `target` S'EN VA. Le lien garde son adresse : un clic du milieu
// ou Ctrl+clic ouvre toujours une vraie fenêtre du navigateur, ce que l'humain
// attend d'un lien, et le clic normal va à l'onglet. Un `href="javascript:void(0)"`
// aurait volé ce geste et rendu l'élément méconnaissable pour un lecteur d'écran.
//
// ⚠️ `target="_blank"` DOIT PARTIR, sinon le preventDefault ne sert à rien sur les
// autres chemins (clic du milieu, « ouvrir dans un nouvel onglet » du menu). Le
// `rel="noopener"` d'origine est inutile une fois le target parti — il ne protège
// plus rien, il n'y a plus de fenêtre liée.
function tabRaccorderLeLien() {
	var lien = document.querySelector('#mem-block a[href="/semantic-memory"]');
	if (!lien || lien.dataset.onglet === '1') return;
	lien.dataset.onglet = '1';
	lien.removeAttribute('target');
	lien.addEventListener('click', function (ev) {
		if (ev.ctrlKey || ev.metaKey || ev.shiftKey || ev.button !== 0) return;
		ev.preventDefault();
		goTab('memory');
	});
}

// tabVeilleLeLien : le lien n'est pas là au premier tour de script. #mem-block vit
// dans une modale, et les groupes `gated-group` sont masqués/rendus par
// 05-status.js selon l'état de la mémoire. On regarde donc le DOM, une fois, sans
// en faire une boucle : MutationObserver ne sonde pas, il est prévenu.
var tabObserveur = null;

function tabVeilleLeLien() {
	if (typeof MutationObserver === 'undefined') {
		// Pas d'observateur (vieux moteur) : on se contente de l'état du moment.
		setTabsVisible();
		return;
	}
	if (tabObserveur) return;
	var racine = document.getElementById('mem-modal') || document.body;
	tabObserveur = new MutationObserver(function () {
		var lien = document.querySelector('#mem-block a[href="/semantic-memory"]');
		if (lien && lien.dataset.onglet !== '1') tabRaccorderLeLien();
		setTabsVisible();
	});
	tabObserveur.observe(document.body, { childList: true, subtree: true, attributes: true, attributeFilter: ['class', 'style'] });
	tabRaccorderLeLien();
	setTabsVisible();
}

// Raccourcis clavier : Alt+1 la conversation, Alt+2 la mémoire. ⚠ Alt et non
// Ctrl/Command : sur macOS, Command+1…9 sont pris par les onglets du NAVIGATEUR
// (Ctrl sur Windows/Chrome, même effet). Un raccourci qui lutte contre le
// navigateur n'est jamais tenu par l'humain. Alt est libre ici, et Alt+1/2 sont
// dans le registre des touches qu'aucun champ de saisie n'utilise.
function tabRaccourcis(ev) {
	if (!ev.altKey || ev.ctrlKey || ev.metaKey || ev.shiftKey) return;
	if (ev.key === '1') { ev.preventDefault(); goTab('chat'); }
	else if (ev.key === '2') { ev.preventDefault(); goTab('memory'); }
}

// tabRelireLaMemoire : demande au serveur si la mémoire sémantique est de service.
//
// ⚠ UNE SEULE FOIS, pas de sonde : c'est un avis, pas un état qui bouge tout seul.
// Un appel raté (réseau court, serveur qui redémarre) ne DOIT PAS griser la barre
// que l'observateur aurait déjà levée : on ne touche à `tabMemoireDispo` que si la
// réponse est lisible, et `setTabsVisible` reste appelé quoi qu'il arrive pour ne
// pas laisser la barre dans un état qu'on n'aurait pas recalculé.
async function tabRelireLaMemoire() {
	try {
		var ctrl = new AbortController();
		var delai = setTimeout(function () { try { ctrl.abort(); } catch (e) {} }, 4000);
		var r = await fetch('/api/embeddings', { cache: 'no-store', signal: ctrl.signal });
		clearTimeout(delai);
		if (!r.ok) return;
		var j = await r.json();
		tabMemoireDispo = !!(j && j.config && j.config.enabled);
	} catch (e) {
		// Silent : l'observateur et le lien restent le second chemin, et une panne
		// de lecture n'est pas une interdiction (même règle qu'en tête de fichier
		// pour le HEAD d'en-cadrement).
	}
	setTabsVisible();
}

function initOnglets() {
	tabVeilleLeLien();
	tabRelireLaMemoire();
	document.addEventListener('keydown', tabRaccourcis);
	// La langue peut changer sans recharger (setLang réapplique applyI18n) : les
	// libellés des onglets portent data-i18n, donc applyI18n les relit seul. Rien
	// de plus à faire ici — c'est pour ça que je ne traduis pas à la main.
}

if (document.readyState === 'loading') {
	document.addEventListener('DOMContentLoaded', initOnglets);
} else {
	initOnglets();
}
