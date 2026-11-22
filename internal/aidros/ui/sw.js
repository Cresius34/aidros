// AJEAN service worker — UNIQUEMENT les notifications Web Push.
//
// ⚠️ VOLONTAIREMENT sans cache ni handler `fetch` : mettre l'app en cache
// casserait l'anti-cache du portail E2E (app.ajean.link sert app.html?v=<build>
// avec un bootstrap qui doit toujours repartir du réseau — voir ajean-app). Un
// service worker qui interposerait une réponse en cache pourrait servir une UI
// périmée et bloquer la récupération de la clé E2E. Ce worker ne fait donc QUE
// recevoir les push du serveur et afficher la notification.
//
// Le SERVEUR (push.go) pousse à la fin d'un tour utilisateur, même app fermée /
// iPhone verrouillé — c'est tout l'intérêt par rapport à une notif côté page.

self.addEventListener('install', function(){ self.skipWaiting(); });
self.addEventListener('activate', function(e){ e.waitUntil(self.clients.claim()); });

self.addEventListener('push', function(e){
  var data = { title: 'Aidros', body: 'Réponse prête' };
  try { if (e.data) data = Object.assign(data, e.data.json()); } catch (_){}
  e.waitUntil(self.registration.showNotification(data.title, {
    body: data.body,
    // tag + renotify : une nouvelle réponse REMPLACE l'ancienne notif (pas
    // d'empilement), mais re-sonne/vibre pour signaler qu'elle est fraîche.
    tag: data.tag || 'ajean-turn',
    renotify: true,
    // Icône = la marque, en data-URI (aucun fichier externe, compatible avec
    // l'origine GitHub Pages du portail E2E). Son dessin vient de
    // sys_brand_icon.go : l'assembleur le pose sur la ligne ci-dessous, comme le
    // favicon et le voile de chargement. (Écrit sans le marqueur ici : le
    // générateur remplace TOUTE occurrence de son marqueur, commentaire compris.)
    icon: "data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 12 12'><rect width='12' height='12' rx='2' ry='2' fill='%23000'/><circle cx='6' cy='6' r='3.7' fill='none' stroke='%23fff' stroke-width='2'/><circle cx='8.350000000000001' cy='1.9296806022131383' r='1.02' fill='%23fff'/><circle cx='9.245470267513111' cy='2.4466806022131387' r='0.62' fill='%23fff'/><circle cx='8.350000000000001' cy='10.07031939778686' r='1.02' fill='%23fff'/><circle cx='7.4545297324868915' cy='10.58731939778686' r='0.62' fill='%23fff'/><circle cx='1.2999999999999998' cy='6.000000000000001' r='1.02' fill='%23fff'/><circle cx='1.2999999999999996' cy='4.966000000000001' r='0.62' fill='%23fff'/></svg>"
  }));
});

// Clic sur la notif : ramène l'onglet AJEAN au premier plan s'il est déjà ouvert,
// sinon en ouvre un. `includeUncontrolled` : les onglets ouverts AVANT que ce
// worker prenne le contrôle comptent aussi.
self.addEventListener('notificationclick', function(e){
  e.notification.close();
  e.waitUntil(self.clients.matchAll({ type: 'window', includeUncontrolled: true }).then(function(cl){
    for (var i = 0; i < cl.length; i++){ if ('focus' in cl[i]) return cl[i].focus(); }
    if (self.clients.openWindow) return self.clients.openWindow('/');
  }));
});
