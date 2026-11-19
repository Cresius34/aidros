package ajean

// Le pilotage à distance d'un Ajean par un AUTRE Ajean, par le pont (18190).
//
// POURQUOI SOUS /v1/. La garde du pont (parallel_bridge.go:168) écarte par un 404
// tout ce qui n'est pas /v1/*, et son commentaire le dit exprès : « ni /api/*
// (pilotage, mémoire, fichiers, agent) ». Un /api/switch envoyé au pont répond
// donc « surface OpenAI uniquement », même avec la bonne clé. Le dossier
// d'échange avait déjà réglé la même question le 17/11/2026 (parallel_bridge_files.go:58) :
// une route du pont doit HABITER sous /v1/, pas espérer une entorse à la garde.
//
// POURQUOI UN INTERRUPTEUR SÉPARÉ, ÉTEINT PAR DÉFAUT. Basculer un preset n'est
// pas lire : cela ARRÊTE le moteur en place. Mesuré chez la voisine le 17/11, le
// 9B et Strata se disputent le même port 18080 — un seul moteur par instance
// (parallel_config.go:83). Un voisin qui clique au mauvais endroit éteint le
// cerveau de la machine, et avec lui le pont qui porte la conversation. C'est le
// même degré de gravité que prêter le shell : cela se décide machine par machine,
// comme parallelBridgeAgentKey.
//
// CE QUE LE COUP D'ARRÊT COÛTE AU PONT LUI-MÊME : serviceAction("restart")
// relance l'application, donc l'instance pilotée perd son pont le temps de
// redémarrer. La réponse part AVANT (go func + sendJSON, comme handleSwitch),
// l'appelant reçoit donc 200 puis le pont s'éteint une vingtaine de secondes.
// C'est voulu, et c'est écrit dans le message rendu pour qu'on ne le découvre
// pas en croyant avoir perdu le voisin.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

// parallelBridgeControlKey : autoriser un voisin à basculer le preset actif et à
// allumer/éteindre l'agent de CETTE machine. ÉTEINT par défaut.
const parallelBridgeControlKey = "parallel_bridge_control"

// parallelBridgeControlOn : le pont a-t-il le droit de piloter cette machine ?
// AJEAN_BRIDGE_CONTROL=1 l'allume pour un test ou un lanceur, sans écrire en base
// (même règle que AJEAN_BRIDGE_AGENT, parallel_bridge.go:86).
func parallelBridgeControlOn() bool {
	if v := strings.TrimSpace(os.Getenv("AJEAN_BRIDGE_CONTROL")); v != "" {
		return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "on")
	}
	return getBool(bkState, parallelBridgeControlKey)
}

func setParallelBridgeControlOn(on bool) error {
	return putBool(bkState, parallelBridgeControlKey, on)
}

// piloteCanal : QUI a donné cet ordre, en deux dimensions — la machine d'où il
// vient (canalFromHeader) et QUI l'a écrit (canalOrigine : « agent » ou « humain »).
//
// Je n'invente PAS d'en-tête pour ça. Ma première version posait un
// X-Ajean-Pilotage dont la valeur était une prose libre venue du réseau, que
// j'imprimais dans mon journal. Or mon propre code énonce la règle pour un autre
// en-tête (parallel_bridge_agent.go:92-96) : « un en-tête HTTP non ASCII est une
// source de bugs, et une raison en prose fuirait vers n'importe quel client ».
// J'ai fait exactement ce que mon commentaire interdit, à quatre fichiers de lui.
// Ici les deux valeurs sont courtes, connues, et déjà posées par peerAsk (L308)
// et peerOrigineCLI (L489) : rien de neuf à valider.
func piloteCanal(r *http.Request) (machine, auteur string) {
	machine = strings.TrimSpace(r.Header.Get(canalFromHeader))
	if machine == "" {
		machine = "machine voisine"
	}
	return machine, canalOrigine(r)
}

// controlRoute : la surface de pilotage. Sous /v1/, pour la même raison que
// echangeRoute : au-dessus de la garde, elle n'existerait pas.
func controlRoute(p string) bool {
	switch {
	case p == "/v1/presets", strings.HasPrefix(p, "/v1/presets/"):
		return true
	case p == "/v1/mode":
		return true
	}
	return false
}

// handleControl : l'aiguillage. Branché dans parallel_bridge.go à côté de
// echangeRoute, donc APRÈS le contrôle de clé (L174/L181) et AVANT canalDemande :
// placé après le canal, un POST de pilotage serait avalé comme un message d'agent.
func handleControl(w http.ResponseWriter, r *http.Request) {
	if !parallelBridgeControlOn() {
		sendJSON(w, http.StatusForbidden, map[string]any{
			"ok":    false,
			"error": "le pilotage à distance est éteint sur ce poste. L'allumer : ajean bridge control on, ou le menu Pont (le basculement d'un preset arrête le moteur en place)",
		})
		return
	}
	switch {
	case r.URL.Path == "/v1/presets" && r.Method == http.MethodGet:
		handleControlPresets(w)
	case r.URL.Path == "/v1/presets/switch" && r.Method == http.MethodPost:
		handleControlSwitch(w, r)
	case r.URL.Path == "/v1/mode" && r.Method == http.MethodGet:
		handleControlModeGet(w)
	case r.URL.Path == "/v1/mode" && r.Method == http.MethodPost:
		handleControlModePost(w, r)
	default:
		sendJSON(w, http.StatusMethodNotAllowed, map[string]any{
			"ok":    false,
			"error": "méthode non admise : GET /v1/presets · POST /v1/presets/switch · GET|POST /v1/mode",
		})
	}
}

// handleControlPresets : la liste, dans la forme dont le voisin a besoin pour
// choisir. handlePresets rend beaucoup plus de champs (bench, quant, ctx) pour
// l'interface ; ici on veut l'essentiel et surtout le millésime du moteur MoE,
// parce que c'est LUI qui fait qu'un preset démarrera ou sera refusé : un exe
// épinglé ne peut pas voir un autre millésime (backend_moe_strata.go:46, mesuré
// le 17/11 sur la voisine — elle avait renommé le mauvais des deux presets).
func handleControlPresets(w http.ResponseWriter) {
	list, err := ListPresets()
	if err != nil {
		sendJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	out := []map[string]any{}
	for _, p := range list {
		item := map[string]any{"id": p.ID, "name": p.Name, "active": p.Active}
		if contenu, err := ReadPreset(p.ID); err == nil {
			cfg := parseEnv(contenu)
			switch {
			case isMoeConfig(cfg):
				item["moe"] = true
				// LE CHAMP QUI ÉVITE LE PIÈGE DU 17/11 : le nom affiché ne dit pas
				// le millésime, et c'est ce qui a fait perdre quatre tours.
				if v := moeMillésimeDe(cfg); v != "" {
					item["moteur"] = v
				}
			case isExternalConfig(cfg):
				item["external"] = true
				if m := strings.TrimSpace(cfg[extKeyModel]); m != "" {
					item["modele"] = m
				}
			}
		}
		out = append(out, item)
	}
	sendJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"presets": out,
		// Ce que le voisin doit savoir avant de cliquer : le moteur épinglé par
		// CET exe. Un preset qui pointe un autre millésime sera refusé, pas démarré.
		"moteur_epingle": strataVersion,
		"attention":      "basculer un preset arrête le moteur en place et redémarre l'application : le pont de ce poste s'éteint le temps du redémarrage",
	})
}

// moeMillésimeDe : le millésime que ce preset va chercher, lu dans ses chemins
// MOE_CONFIG/MOE_PYTHON/MOE_SOURCE. C'est LA chose que l'interface ne montre pas
// (elle montre le # NAME du fichier) et qui a coûté quatre tours de diagnostic
// chez la voisine le 17/11 : deux presets GYRO-S, un seul pointait strata-0.1.57-hip.
func moeMillésimeDe(cfg map[string]string) string {
	for _, k := range []string{"MOE_CONFIG", "MOE_PYTHON", "MOE_SOURCE"} {
		if v := moeMillésimeDuChemin(cfg[k]); v != "" {
			return v
		}
	}
	return ""
}

// moeMillésimeDuChemin : « …\moe\strata-0.1.57-hip\serve\server.py » → « 0.1.57 ».
// Le motif « strata-<version>-<suffixe> » est celui que mon installeur écrit
// lui-même (il dérive le cache du manifeste, variante-rx6900xt), donc le lire
// n'est pas deviner un nom d'utilisateur : c'est relire ce que j'ai posé.
func moeMillésimeDuChemin(chemin string) string {
	c := strings.ToLower(strings.TrimSpace(chemin))
	if c == "" {
		return ""
	}
	c = strings.ReplaceAll(c, "/", "\\")
	for _, seg := range strings.Split(c, "\\") {
		if !strings.HasPrefix(seg, "strata-") {
			continue
		}
		reste := strings.TrimPrefix(seg, "strata-")
		// le millésime est le premier segment avant un tiret ou la fin
		if i := strings.IndexByte(reste, '-'); i > 0 {
			reste = reste[:i]
		}
		reste = strings.TrimSuffix(reste, "-hip")
		if reste != "" {
			return reste
		}
	}
	return ""
}

// handleControlSwitch : on désigne le preset par son IDENTIFIANT, jamais par sa
// position. C'est la leçon de handleSwitch (web_api.go:1311) : « le numéro seul
// est dangereux : si la liste a changé depuis l'affichage, la même position
// désigne un AUTRE preset ». À distance, la liste a d'autant plus changé que
// c'est nous qui l'avons peut-être modifiée.
func handleControlSwitch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	id := strings.TrimSpace(req.ID)
	if id == "" {
		sendJSON(w, http.StatusBadRequest, map[string]any{
			"ok": false, "error": "aucun preset désigné : donner son id (GET /v1/presets le rend)",
		})
		return
	}
	machine, auteur := piloteCanal(r)
	nom, err := switchToPresetByID(id)
	if err != nil {
		sendJSON(w, http.StatusNotFound, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	// Trace locale : QUI a changé le preset de cette machine, et si c'était une
	// personne ou une IA. Sans ça, un matin le moteur ne serait plus celui d'hier
	// et personne ne saurait d'où ça vient — ni si c'est un ordre ou une initiative.
	fmt.Printf("%s pilotage du pont : %s (poste « %s ») a basculé le preset sur %s\n",
		green("[ok]"), auteur, machine, nom)
	sendJSON(w, http.StatusOK, map[string]any{
		"ok": true, "preset": nom,
		"effet": "le moteur en place est arrêté, l'application redémarre : le pont de ce poste coupe quelques secondes",
	})
}

func handleControlModeGet(w http.ResponseWriter) {
	sendJSON(w, http.StatusOK, map[string]any{
		"ok": true,
		// Les deux niveaux de l'agent : l'autorisation du pont ET le mode agent de
		// la machine. L'un sans l'autre ne déclenche rien (mesuré, parallel_bridge.go:825).
		"agent":       parallelBridgeAgentOn(),
		"agent_local": agentEnabled(),
		"pilotage":    parallelBridgeControlOn(),
		"declenchable": parallelBridgeAgentOn() && agentEnabled(),
	})
}

// handleControlModePost : allumer/couper l'agent du voisin à distance. C'est la
// moitié de la demande : « basculer les presets/mode d'un autre Ajean ».
func handleControlModePost(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Agent string `json:"agent"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	// `demande` et non `qui` : ce mot décrit la VALEUR demandée ("on"/"off"), pas la
	// personne. Ma première version nommait les deux `qui`, dans la même fonction,
	// et l'un écrasait l'autre — le compilateur l'a crié, je ne l'avais pas vu.
	demande := strings.ToLower(strings.TrimSpace(req.Agent))
	var on bool
	switch demande {
	case "on", "activer", "oui", "1", "vrai", "true":
		on = true
	case "off", "couper", "non", "0", "faux", "false":
		on = false
	default:
		sendJSON(w, http.StatusBadRequest, map[string]any{
			"ok": false, "error": `agent : "on" ou "off"`,
		})
		return
	}
	machine, auteur := piloteCanal(r)
	if err := setParallelBridgeAgentOn(on); err != nil {
		sendJSON(w, http.StatusServiceUnavailable, map[string]any{"ok": false, "error": err.Error()})
		return
	}
	reponse := map[string]any{"ok": true, "agent": on}
	if on && !agentEnabled() {
		// Le voisin vient d'ouvrir une porte que la machine tient fermée de l'autre
		// côté : le lui DIRE, sinon il appellera en croyant que ça marche.
		reponse["attention"] = "l'autorisation du pont est prise, mais le mode agent de cette machine est éteint : rien ne se déclenchera tant que l'interface n'est pas en mode Projet/Agent"
	}
	fmt.Printf("%s pilotage du pont : agent %s par %s (poste « %s »)\n",
		green("[ok]"), map[bool]string{true: "allumé", false: "éteint"}[on], auteur, machine)
	sendJSON(w, http.StatusOK, reponse)
}
