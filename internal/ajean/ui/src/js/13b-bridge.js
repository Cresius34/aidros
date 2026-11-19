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

let bridgeEtat = null;

function renderBridge(d){
  bridgeEtat = d;
  const card = document.getElementById('bridge-details');
  if(!card || !d || d.ok !== true) return;      // 403/404 : pas de carte ici
  card.hidden = false;

  const live = !!d.listening;
  setBadge('bridge-badge', live ? true : (d.enabled ? 'warn' : false),
           live ? 'à l’écoute' : (d.enabled ? 'à réveiller' : 'éteint'));

  const st = document.getElementById('bridge-state');
  if(st){
    st.textContent = live ? 'en ligne' : (d.enabled ? 'aucune écoute' : 'fermé');
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
  const src = d.serving || ('moteur local 127.0.0.1:' + (d.engine_serving || d.engine_port));
  if(mdl && note){
    if(d.model){
      mdl.textContent = 'Modèle prêté : ' + d.model;
      note.textContent = 'Écoute sur le port ' + d.port + ' ; le cerveau vient de : ' + src + '.';
    } else if(live){
      mdl.textContent = 'Aucun modèle ne répond derrière ce pont';
      note.textContent = 'Le pont écoute mais rien ne répond derrière (' + src + ') : activer un preset — MoE local, API externe ou cloud — sinon l’autre Ajean recevra une erreur.';
    } else {
      mdl.textContent = 'Aucun modèle prêté';
      note.textContent = 'Un pont ouvert sans preset actif n’a rien à prêter : activer un preset (page Modèles), MoE local, API externe ou cloud.';
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
    box.innerHTML = '<div class="rowbody"><p class="muted" style="margin:0">Aucun voisin déclaré. Un canal se déclare à la main, jamais par découverte automatique du réseau.</p></div>';
    return;
  }
  box.innerHTML = peersList.map(function(p, i){
    const cle = p.has_key ? 'clé posée' : '<b>sans clé</b> — il refusera la demande';
    const can = p.thread ? ' · canal ' + p.thread + ' messages' : '';
    return '<div class="switchrow"><span class="sw-txt"><span>' + escHtml(p.name) +
      '</span><span class="sw-sub">' + escHtml(p.url) + ' · ' + cle + can + '</span></span>' +
      '<button type="button" class="sw-action" onclick="peerForgetIdx(' + i + ')">Oublier</button>' +
      '<button type="button" class="sw-action" onclick="peerRemoveIdx(' + i + ')">Retirer</button></div>';
  }).join('');
}

function peerForgetIdx(i){ if(peersList[i]) peerForget(peersList[i].name); }
function peerRemoveIdx(i){ if(peersList[i]) peerRemove(peersList[i].name); }

async function peerAdd(){
  const g = function(id){ const e = document.getElementById(id); return e ? e.value.trim() : ''; };
  const name = g('peer-name'), url = g('peer-url'), key = g('peer-key');
  if(!name || !url){ toast('Nom et adresse du voisin requis'); return; }
  try{
    const r = await jpost('/api/peers', { add: name, url: url, key: key });
    if(!r.ok){ throw new Error(r.error || 'refusé'); }
    ['peer-name','peer-url','peer-key'].forEach(function(id){ const e = document.getElementById(id); if(e) e.value=''; });
    toast('Voisin ' + r.name + ' déclaré');
    loadPeers();
  }catch(e){ toast('Voisin : ' + e.message); }
}

async function peerRemove(name){
  try{
    const r = await jpost('/api/peers', { remove: name });
    if(!r.ok){ throw new Error(r.error || 'refusé'); }
    toast('Voisin ' + name + ' retiré');
    loadPeers();
  }catch(e){ toast('Voisin : ' + e.message); }
}

async function peerForget(name){
  try{
    const r = await jpost('/api/peers', { forget: name });
    if(!r.ok){ throw new Error(r.error || 'refusé'); }
    toast('Souvenir du canal effacé');
    loadPeers();
  }catch(e){ toast('Canal : ' + e.message); }
}

async function bridgeToggle(wanted){
  const box = document.getElementById('bridge-on');
  try{
    const r = await jpost('/api/bridge', { enabled: !!wanted });
    if(!r.ok){ throw new Error(r.error || 'refusé'); }
    renderBridge(r);
    toast(wanted ? 'Pont ouvert sur le réseau local' : 'Pont coupé');
  }catch(e){
    if(box) box.checked = !wanted;
    toast('Pont : ' + e.message);
  }
}

// Porte 2 : le tour d'agent du voisin sur CETTE machine.
async function bridgeSetAgent(wanted){
  const box = document.getElementById('bridge-agent');
  try{
    const r = await jpost('/api/bridge', { agent: !!wanted });
    if(!r.ok){ throw new Error(r.error || 'refusé'); }
    renderBridge(r);
    // Le voisin peut ouvrir cette porte pendant que la machine tient la sienne
    // fermée : le dire tout de suite, sinon la case coche sans effet.
    if(wanted && r.agent_requested && !r.agent_mode_local){
      toast('Autorisation prise, mais le mode Agent de cette machine est éteint : rien ne se déclenchera');
    } else {
      toast(wanted ? 'Agent du voisin autorisé ici' : 'Agent du voisin refusé');
    }
  }catch(e){
    if(box) box.checked = !wanted;
    toast('Agent du voisin : ' + e.message);
  }
}

// Porte 3 : le pilotage. La plus grave des trois — elle permet au voisin
// d'arrêter le moteur de cette machine.
async function bridgeSetControl(wanted){
  const box = document.getElementById('bridge-control');
  try{
    const r = await jpost('/api/bridge', { control: !!wanted });
    if(!r.ok){ throw new Error(r.error || 'refusé'); }
    renderBridge(r);
    toast(wanted ? 'Le voisin peut changer le moteur de cette machine'
                 : 'Pilotage coupé — le voisin ne reçoit plus que le modèle nu');
  }catch(e){
    if(box) box.checked = !wanted;
    toast('Pilotage : ' + e.message);
  }
}

async function bridgeSavePort(){
  const inp = document.getElementById('bridge-port');
  const p = ((inp && inp.value) || '').trim();
  try{
    const r = await jpost('/api/bridge', { port: p });
    if(!r.ok){ throw new Error(r.error || 'refusé'); }
    renderBridge(r);
    toast('Port du pont : ' + r.port);
  }catch(e){
    toast('Port du pont : ' + e.message);
    if(inp && bridgeEtat) inp.value = bridgeEtat.port || '';
  }
}
