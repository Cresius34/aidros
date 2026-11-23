// 00b-i18n.js — moteur de traduction minimal, sans dépendance, cohérent avec
// le reste de cette UI (pas de module, tout en portée globale, un seul script
// concaténé par tools/assemble-ui — voir assemble-ui/main.go).
//
// Principe : chaque élément statique du HTML porte data-i18n="clé". Au chargement
// (et à chaque changement de langue), applyI18n() parcourt ces éléments et pose
// le texte de la langue active. Le contenu FRANÇAIS déjà écrit dans le HTML sert
// de secours si une clé manque dans une langue (y compris si elle manque en
// français : ça ne devrait jamais arriver, mais mieux vaut un texte que rien).
//
// Ajouter une langue : dupliquer un bloc dans I18N (00a-i18n-data.js), traduire
// les valeurs, ajouter son code à LANG_NAMES ci-dessous et à la liste déroulante
// dans index.tmpl.html (#lang-select) — trois endroits, aucun autre changement.
//
// Pour le JS qui construit du texte dynamiquement (toasts, contenu injecté),
// utiliser t('clé') directement plutôt que data-i18n.

// Seules les langues listées ici sont proposées dans le sélecteur (#lang-select)
// et acceptées par currentLang(). L'anglais est la langue par défaut ; il vient
// en premier pour être la première option de la liste. Les blocs it/es/ru/de
// amorcés dans I18N (00a-i18n-data.js) restent volontairement inertes tant
// qu'ils ne sont pas complets et ajoutés ici.
var LANG_NAMES = {
	en: 'English', fr: 'Français'
};

function currentLang() {
	try {
		var l = localStorage.getItem('ajean-lang');
		if (l && LANG_NAMES[l] && I18N[l]) return l;
	} catch (e) {}
	return 'en';
}

// t(key) renvoie le texte traduit pour la langue active, replie sur le
// français si la clé manque dans cette langue, puis sur la clé elle-même si
// elle manque partout (un texte "cassé" visible vaut mieux qu'une exception).
function t(key) {
	var lang = currentLang();
	var dict = I18N[lang];
	if (dict && Object.prototype.hasOwnProperty.call(dict, key)) return dict[key];
	if (I18N.fr && Object.prototype.hasOwnProperty.call(I18N.fr, key)) return I18N.fr[key];
	return key;
}

// applyI18n pose le texte traduit sur chaque [data-i18n] sous `root` (tout le
// document par défaut). Trois variantes selon les attributs présents sur
// l'élément :
//   data-i18n="clé"                          → textContent
//   data-i18n="clé" data-i18n-html            → innerHTML (texte avec <b>/<br> etc.)
//   data-i18n="clé" data-i18n-attr="title"    → pose l'ATTRIBUT nommé plutôt que le contenu
function applyI18n(root) {
	root = root || document;
	var nodes = root.querySelectorAll('[data-i18n]');
	for (var i = 0; i < nodes.length; i++) {
		var el = nodes[i];
		var key = el.getAttribute('data-i18n');
		var val = t(key);
		var attr = el.getAttribute('data-i18n-attr');
		if (attr) {
			el.setAttribute(attr, val);
		} else if (el.hasAttribute('data-i18n-html')) {
			el.innerHTML = val;
		} else {
			el.textContent = val;
		}
	}
	try { document.documentElement.setAttribute('lang', currentLang()); } catch (e) {}
}

// setLang change la langue active, réapplique aussitôt (pas de rechargement de
// page) et synchronise avec /api/prefs (savePrefs, définie dans 02-prefs.js)
// si elle est déjà chargée — même schéma que le thème sombre/clair. applyI18n()
// ne suffit pas seule : elle ne touche que les éléments statiques marqués
// data-i18n. Tout ce qui est reconstruit en JS avec t() au moment du rendu
// (panneaux Tâches, Moteur/Config, Agent, Internet, MCP, Réseau, clé API…)
// garde l'ancien texte tant qu'il ne se re-rend pas — d'où le loadAll() ici
// pour forcer un rafraîchissement immédiat de tout ce contenu dynamique.
function setLang(lang) {
	if (!I18N[lang]) return;
	try { localStorage.setItem('ajean-lang', lang); } catch (e) {}
	applyI18n();
	if (typeof savePrefs === 'function') savePrefs();
	if (typeof loadAll === 'function') loadAll();
	// Le nom du projet par défaut se dit dans la langue (projects.default_name).
	// loadAll() ne le repeint pas : il retouche les panneaux, pas la bulle du
	// composeur ni l'en-tête de l'historique.
	//
	// ⚠ RE-RENDU LOCAL, PAS UN RECHARGEMENT. PROJECTS est déjà en mémoire, le
	// libellé se recalcule sans le serveur. Je n'appelle PAS loadProjects() ni
	// loadHistory() ici, pour deux raisons nommées :
	//   1. le nombre de chargeurs de loadAll() est un contrat gardé par un test
	//      (ui_reserve_hauteur_test.go : « 17 chargeurs ») ;
	//   2. loadHistory() reconstruit tout le fil — mesuré 148 108 jetons le
	//      23/11 — et le payer pour un seul mot serait un changement de langue
	//      plus lent que l'ouverture d'un tchat.
	// CE QUI RESTE TRADUIT PLUS TARD, dit franchement : l'étiquette de projet
	// d'une ligne d'historique déjà affichée (projetLabel, 08-chat-render.js)
	// prend la nouvelle langue au prochain rechargement de l'historique, pas
	// à la seconde. C'est une ligne de liste, pas l'écran courant.
	if (typeof setProjectBtnName === 'function' && typeof PROJECTS !== 'undefined'
		&& typeof ACTIVE_PROJECT !== 'undefined') {
		var act = (PROJECTS || []).find(function (p) { return p.slug === ACTIVE_PROJECT; });
		if (typeof projLabel === 'function') setProjectBtnName(projLabel(act));
	}
	if (typeof histHeadLabel === 'function') histHeadLabel();
	if (typeof refreshProjMenu === 'function') refreshProjMenu();
}

document.addEventListener('DOMContentLoaded', function () {
	var sel = document.getElementById('lang-select');
	if (sel) {
		for (var code in LANG_NAMES) {
			if (!Object.prototype.hasOwnProperty.call(LANG_NAMES, code)) continue;
			var opt = document.createElement('option');
			opt.value = code;
			opt.textContent = LANG_NAMES[code];
			sel.appendChild(opt);
		}
		sel.value = currentLang();
	}
	applyI18n();
});
// tf : traduction avec repli. Rend le texte fourni si la clé n'existe pas (une
// clé absente est renvoyée telle quelle par t()).
function tf(key, fr){ const v = t(key); return v === key ? fr : v; }

// tp : traduction TROUEE. La valeur peut porter {0}, {1}, ... ; tp les remplace
// dans l'ordre par les arguments fournis.
//
// POURQUOI cette fonction, et pas des préfixes collés à la main (« Écoute sur le
// port » + port + « ; le cerveau vient de : » + source) : l'ordre des morceaux
// n'est pas le même dans les deux langues. Une phrase coupée en préfixes rend un
// anglais compréhensible et un français bancal, ou l'inverse, selon la langue.
// Le trou est dans la PHRASE, la phrase reste une seule valeur à traduire.
//
// ⚠ Le remplacement est littéral, jamais une expression régulière : un port ou
// une URL ne peut pas casser le texte. Les arguments sont convertis en chaîne
// sans échappement HTML — appeler escHtml() soi-même si le résultat va dans
// innerHTML (c'est déjà le cas partout où ce fichier le fait).
function tp(key) {
	var v = t(key);
	for (var i = 1; i < arguments.length; i++) {
		v = v.split('{' + (i - 1) + '}').join(String(arguments[i] == null ? '' : arguments[i]));
	}
	return v;
}
