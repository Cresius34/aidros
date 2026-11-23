// Pont Ajean↔Ajean — prêter le cerveau de cette machine à un autre Ajean du
// même réseau local. Réservé à la version portable : /api/bridge répond 403
// ailleurs, et la carte reste cachée dans ce cas. Éteint par défaut côté Go ;
// cette carte ne va jamais allumer quelque chose d'elle-même, elle lit l'état.
//
// Ce que le pont expose : uniquement /v1/* (l'API du modèle), verrouillé par la
// clé API de la machine. Ni la mémoire, ni le chat.
//
// TROIS PORTES, trois réglages, parce que trois gravités différentes. Les
// confondre est ce qui a fait chercher midi à quatorze heures le 17/11 : une
// route qui répond 403 pour la mauvaise raison ne se débusque pas toute seule.
//   1. le pont lui-même  — prêter le modèle (complétions seules)
//   2. l'agent du voisin — autoriser un tour d'agent ICI : shell, disque, mémoire
//   3. le pilotage       — autoriser le voisin à CHANGER cette machine : son
//      preset actif, son agent. La plus grave : basculer un preset arrête le
//      moteur en place, et sur un poste où texte et image se partagent un port,
//      le mauvais clic éteint le cerveau — et le pont qui porte la conversation.
//
// ET DANS L'AUTRE SENS (19/11) : piloter le voisin. La carte « Parler à un autre
// Ajean » a un bouton par voisin qui lit SES presets et permet d'en activer un.
// Le trajet passe par /api/peers/pilot, jamais par le navigateur : la clé du
// voisin ne sort pas du magasin local (GET /api/peers ne rend que has_key).

let bridgeEtat = null;

function renderBridge(d){
  bridgeEtat = d;
  const card = document.getElementById('bridge-details');
  if(!card || !d || d.ok !== true) return;      // 403/404 : pas de carte ici
  card.hidden = false;

  const live = !!d.listening;
  setBadge('bridge-badge', live ? true : (d.enabled ? 'warn' : false),
           live ? t('bridge.badge_listening') : (d.enabled ? t('bridge.badge_sleep') : t('bridge.badge_off')));

  const st = document.getElementById('bridge-state');
  if(st){
    st.textContent = live ? t('bridge.state_online') : (d.enabled ? t('bridge.state_silent') : t('bridge.state_closed'));
    st.classList.toggle('on', live);
  }
  const on = document.getElementById('bridge-on');
  if(on) on.checked = !!d.enabled;

  // Les portes s'affichent par SOURCE, pas par effet. agent_on côté Go est le ET
  // de deux sources ; cocher la case pendant que le mode Agent de la machine est
  // éteint ne déclenche rien (mesuré, parallel_bridge.go:825). Afficher la source
  // évite une case qui semble allumée sans effet.
  const ag = document.getElementById('bridge-agent');
  if(ag) ag.checked = !!d.agent_requested;
  const ct = document.getElementById('bridge-control');
  if(ct) ct.checked = !!d.control_on;

  const port = document.getElementById('bridge-port');
  if(port) port.value = d.port || '';
  const url = document.getElementById('bridge-url');
  if(url) url.value = (d.urls && d.urls.length && live) ? d.urls[0] : '';

  // Ce que le pont prêtera réellement — et d'OÙ il le prend. Le pont ne dépend
  // d'aucun moteur en particulier : il prête le preset actif, qu'il soit un
  // llama-server local, une API distante ou un GPU cloud. Les deux causes de
  // « ouvert mais injoignable » sont dites ici plutôt que laissées à deviner :
  // pas de clé (le pont refuse tout le monde) et rien derrière (pas de cerveau).
  const mdl = document.getElementById('bridge-model');
  const note = document.getElementById('bridge-note');
  const src = d.serving || tp('bridge.engine_local', d.engine_serving || d.engine_port);
  if(mdl && note){
    if(d.model){
      mdl.textContent = tp('bridge.model_lent', d.model);
      note.textContent = tp('bridge.note_listening', d.port, src);
    } else if(live){
      mdl.textContent = t('bridge.model_dead');
      note.textContent = tp('bridge.note_dead', src);
    } else {
      mdl.textContent = t('bridge.model_none');
      note.textContent = t('bridge.note_no_preset');
    }
  }
}

async function loadBridge(){
  try{
    const r = await jfetch('/api/bridge');
    if(r.status === 403 || r.status === 404) return;   // pas ce build
    const d = await r.json();
    renderBridge(d);
  }catch(e){ /* carte absente de cette version : on n'en parle pas */ }
  loadPeers();
}

// ---- le canal : les voisins déclarés -------------------------------------
// Un voisin n'est PAS un modèle emprunté. C'est une autre IA, avec son propre
// cerveau et sa propre mémoire, joignable par-dessus le pont. La liste ne montre
// jamais la clé — seulement si elle est posée.
let peersList = [];

async function loadPeers(){
  const block = document.getElementById('peers-block');
  if(!block) return;
  try{
    const r = await jfetch('/api/peers');
    if(r.status === 403 || r.status === 404){ return; }
    const d = await r.json();
    peersList = d.peers || [];
    block.hidden = false;
    renderPeers();
  }catch(e){ /* pas de canal dans cette version */ }
}

function renderPeers(){
  const box = document.getElementById('peers-list');
  if(!box) return;
  if(!peersList.length){
    box.innerHTML = '<div class="rowbody"><p class="muted" style="margin:0">' + escHtml(t('bridge.no_peers')) + '</p></div>';
    return;
  }
  box.innerHTML = peersList.map(function(p, i){
    // La clé est le seul élément qui porte du balisage : le reste passe par
    // escHtml, sinon le nom d'un voisin ou son URL se retrouvent interprétés.
    const cle = p.has_key ? escHtml(t('bridge.key_set')) : t('bridge.key_missing');
    const can = p.thread ? tp('bridge.thread_count', p.thread) : '';
    return '<div class="switchrow"><span class="sw-txt"><span>' + escHtml(p.name) +
      '</span><span class="sw-sub">' + escHtml(p.url) + ' · ' + cle + can + '</span></span>' +
      // Piloter le voisin : lire SES moteurs, en activer un. Le bouton ne fait
      // qu'afficher le panneau — aucun appel réseau à l'affichage du menu, vers
      // une machine qui peut être éteinte.
      '<button type="button" class="sw-action" onclick="peerPilotToggleIdx(' + i + ')">' + escHtml(t('bridge.peers_engines')) + '</button>' +
      '<button type="button" class="sw-action" onclick="peerForgetIdx(' + i + ')">' + escHtml(t('bridge.peer_forget')) + '</button>' +
      '<button type="button" class="sw-action" onclick="peerRemoveIdx(' + i + ')">' + escHtml(t('bridge.peer_remove')) + '</button></div>' +
      '<div class="pilot-block" id="pilot-' + i + '" hidden></div>';
  }).join('');
}

function peerForgetIdx(i){ if(peersList[i]) peerForget(peersList[i].name); }
function peerRemoveIdx(i){ if(peersList[i]) peerRemove(peersList[i].name); }

// ---- piloter le voisin : ses moteurs, et en activer un -------------------
//
// POURQUOI AU CLIC, et pas au chargement du menu : le voisin peut être éteint.
// Un appel à l'ouverture de la page ferait une erreur rouge pour une simple
// lecture de réglages locaux.
//
// LE MILLÉSIME EST LA PIÈCE QUI COMPTE. Chez la voisine le 17/11, deux presets
// GYRO-S, l'interface n'affichait que le # NAME du fichier — « FLASH NEXT GYRO-S
// HIP » et « FLASH NEXT GYRO-S HIP 1.48 ». Elle a choisi le mauvais, qui pointait
// strata-0.1.48-hip, et son moteur était refusé. Quatre tours perdus parce
// qu'aucun écran ne disait le millésime. Ici il vient du poste qui le sait.
let pilotOuvert = -1;

function peerPilotToggleIdx(i){
  const box = document.getElementById('pilot-' + i);
  if(!box || !peersList[i]) return;
  // Un seul panneau ouvert : deux listes de moteurs côte à côte prêtent à
  // cliquer sur le mauvais voisin, ce qui est exactement l'erreur du 17/11.
  if(pilotOuvert === i){
    box.hidden = true; box.innerHTML = ''; pilotOuvert = -1; return;
  }
  if(pilotOuvert >= 0){
    const avant = document.getElementById('pilot-' + pilotOuvert);
    if(avant){ avant.hidden = true; avant.innerHTML = ''; }
  }
  pilotOuvert = i;
  box.hidden = false;
  box.innerHTML = '<div class="rowbody"><p class="muted" style="margin:0">' + escHtml(t('bridge.pilot_reading')) + '</p></div>';
  peerPilotLoad(i);
}

async function peerPilotLoad(i){
  const box = document.getElementById('pilot-' + i);
  if(!box) return;
  const p = peersList[i];
  if(!p) return;
  try{
    const r = await jpost('/api/peers/pilot', { name: p.name, action: 'presets' });
    if(!r.ok){ throw new Error(r.error || t('bridge.refused')); }
    const ps = r.presets || [];
    const epingle = r.moteur_epingle || '';
    let html = '';
    if(!ps.length){
      html = '<div class="rowbody"><p class="muted" style="margin:0">' + escHtml(t('bridge.pilot_none')) + '</p></div>';
    } else {
      html = ps.map(function(pre, j){
        // Le millésime, et s'il correspond au moteur que l'exe du voisin sait
        // lancer. Un preset qui pointe un autre millésime sera REFUSÉ, pas démarré :
        // le dire avant le clic vaut mieux que le découvrir après.
        const moteur = pre.moteur ? tp('bridge.pilot_engine', escHtml(pre.moteur)) : '';
        const doute = pre.moteur && epingle && pre.moteur !== epingle
          ? ' <b class="warn">' + escHtml(tp('bridge.pilot_mismatch', epingle)) + '</b>'
          : '';
        const actif = pre.active ? t('bridge.pilot_active') : '';
        const kind = pre.external ? t('bridge.kind_api') : (pre.moe ? t('bridge.kind_moe') : t('bridge.kind_preset'));
        return '<div class="switchrow"><span class="sw-txt"><span>' + escHtml(pre.name || pre.id) +
          actif + '</span><span class="sw-sub">' + kind + moteur + doute +
          '</span></span>' +
          (pre.active ? '' : '<button type="button" class="sw-action" onclick="peerPilotSwitchIdx(' + i + ',' + j + ')">' + escHtml(t('bridge.pilot_activate')) + '</button>') +
          '</div>';
      }).join('');
    }
    // L'avertissement, dans la panneau et pas seulement dans le toast : l'acte
    // est grave et irréversible en un clic.
    html += '<div class="rowbody"><p class="muted" style="margin:0">' + escHtml(t('bridge.pilot_warning')) + '</p></div>';
    box.innerHTML = html;
    // La liste est gardée pour que le clic trouve l'id, jamais une position
    // périmée : si le voisin a modifié ses presets depuis, la position désignerait
    // un autre preset.
    box._presets = ps;
  }catch(e){
    box.innerHTML = '<div class="rowbody"><p class="muted" style="margin:0">' + escHtml(tp('bridge.pilot_unreachable', e.message || t('bridge.error_word'))) + '</p></div>';
  }
}

async function peerPilotSwitchIdx(i, j){
  const box = document.getElementById('pilot-' + i);
  const p = peersList[i];
  if(!box || !p) return;
  const ps = box._presets || [];
  const pre = ps[j];
  if(!pre || !pre.id){ toast(t('bridge.pilot_stale')); return; }
  try{
    const r = await jpost('/api/peers/pilot', { name: p.name, action: 'switch', id: pre.id });
    if(!r.ok){ throw new Error(r.error || t('bridge.refused')); }
    toast(tp('bridge.pilot_switched', p.name, r.preset || pre.id));
    // Le panneau se referme : sa liste est périmée dès que le preset actif a
    // changé, et la rouvrir coûte un appel, pas un risque de clic sur du vieux.
    box.hidden = true; box.innerHTML = ''; box._presets = null;
    if(pilotOuvert === i) pilotOuvert = -1;
  }catch(e){ toast(tp('bridge.peer_error', p.name, e.message)); }
}

async function peerAdd(){
  const g = function(id){ const e = document.getElementById(id); return e ? e.value.trim() : ''; };
  const name = g('peer-name'), url = g('peer-url'), key = g('peer-key');
  if(!name || !url){ toast(t('bridge.peer_need_fields')); return; }
  try{
    const r = await jpost('/api/peers', { add: name, url: url, key: key });
    if(!r.ok){ throw new Error(r.error || t('bridge.refused')); }
    ['peer-name','peer-url','peer-key'].forEach(function(id){ const e = document.getElementById(id); if(e) e.value=''; });
    toast(t('bridge.peer_added', r.name));
    loadPeers();
  }catch(e){ toast(tp('bridge.peer_error_plain', e.message)); }
}

async function peerRemove(name){
  try{
    const r = await jpost('/api/peers', { remove: name });
    if(!r.ok){ throw new Error(r.error || t('bridge.refused')); }
    toast(tp('bridge.peer_removed', name));
    loadPeers();
  }catch(e){ toast(tp('bridge.peer_error_plain', e.message)); }
}

async function peerForget(name){
  try{
    const r = await jpost('/api/peers', { forget: name });
    if(!r.ok){ throw new Error(r.error || t('bridge.refused')); }
    toast(t('bridge.thread_forgotten'));
    loadPeers();
  }catch(e){ toast(tp('bridge.channel_error', e.message)); }
}

async function bridgeToggle(wanted){
  const box = document.getElementById('bridge-on');
  try{
    const r = await jpost('/api/bridge', { enabled: !!wanted });
    if(!r.ok){ throw new Error(r.error || t('bridge.refused')); }
    renderBridge(r);
    toast(wanted ? t('bridge.opened') : t('bridge.closed'));
  }catch(e){
    if(box) box.checked = !wanted;
    toast(tp('bridge.bridge_error', e.message));
  }
}

// Porte 2 : le tour d'agent du voisin sur CETTE machine.
async function bridgeSetAgent(wanted){
  const box = document.getElementById('bridge-agent');
  try{
    const r = await jpost('/api/bridge', { agent: !!wanted });
    if(!r.ok){ throw new Error(r.error || t('bridge.refused')); }
    renderBridge(r);
    // Le voisin peut ouvrir cette porte pendant que la machine tient la sienne
    // fermée : le dire tout de suite, sinon la case coche sans effet.
    if(wanted && r.agent_requested && !r.agent_mode_local){
      toast(t('bridge.agent_no_mode'));
    } else {
      toast(wanted ? t('bridge.agent_allowed') : t('bridge.agent_denied'));
    }
  }catch(e){
    if(box) box.checked = !wanted;
    toast(tp('bridge.agent_error', e.message));
  }
}

// Porte 3 : le pilotage. La plus grave des trois — elle permet au voisin
// d'arrêter le moteur de cette machine.
async function bridgeSetControl(wanted){
  const box = document.getElementById('bridge-control');
  try{
    const r = await jpost('/api/bridge', { control: !!wanted });
    if(!r.ok){ throw new Error(r.error || t('bridge.refused')); }
    renderBridge(r);
    toast(wanted ? t('bridge.control_on') : t('bridge.control_off'));
  }catch(e){
    if(box) box.checked = !wanted;
    toast(tp('bridge.control_error', e.message));
  }
}

async function bridgeSavePort(){
  const inp = document.getElementById('bridge-port');
  const p = ((inp && inp.value) || '').trim();
  try{
    const r = await jpost('/api/bridge', { port: p });
    if(!r.ok){ throw new Error(r.error || t('bridge.refused')); }
    renderBridge(r);
    toast(tp('bridge.port_msg', r.port));
  }catch(e){
    toast(tp('bridge.port_msg', e.message));
    if(inp && bridgeEtat) inp.value = bridgeEtat.port || '';
  }
}
