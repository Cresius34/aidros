package aidros

// Piloter UN VOISIN depuis l'interface : la route locale qui relaie.
//
// La demande de l'humain du 19/11, second volet : « basculer les presets/mode d'un
// AUTRE Ajean », et depuis le menu — pas seulement la ligne de commande.
//
// POURQUOI UNE ROUTE LOCALE, alors que le voisin expose déjà /v1/presets. Deux
// raisons, toutes deux mesurées.
//
//  1. LA CLÉ. La clé du voisin ne sort jamais du magasin local : GET /api/peers ne
//     rend que `has_key` (parallel_peer.go:868), et la carte ne l'affiche pas. La
//     faire transiter par le navigateur l'exposerait dans le DOM, dans l'historique
//     des requêtes, et dans ce que l'extension de l'interface peut lire. Le relais
//     la garde côté Go.
//  2. LE MODÈLE. Le pont d'en face répond sur sa surface /v1 ; un appel direct
//     depuis le navigateur n'aurait pas d'endpoint OpenAI à appeler pour ça, et
//     devrait porter l'en-tête du canal — que le navigateur n'a aucune raison de
//     connaître.
//
// « PRENDRE LA MAIN SANS MODÈLE CHARGÉ », l'exigence du 19/11. Elle tient, et ce
// n'est pas un voeu : startParallelBridge (parallel_bridge.go:486) ne regarde que
// deux choses — le pont demandé, et une clé API. Jamais un moteur. Le routeur place
// controlRoute (L201) AVANT canalDemande (L210) et avant toute complétion : un
// POST /v1/presets/switch ne demande pas une génération, il écrit un réglage. Donc
// un poste dont le modèle est en train de tomber, ou qu'un preset a laissé sans
// moteur, reste pilotable — c'est précisément le moment où l'on veut changer de
// preset. C'est le cas d'école du 17/11 : la voisine avait un mauvais preset
// sélectionné, son moteur ne démarrait pas, et il fallait pouvoir choisir l'autre.
// Le filet TestPilotageSansModeleCharge le joue pour de vrai, pas en intention.
//
// QUI PARLE : peerOrigineCLI(). Dans l'interface, AJEAN_AGENT_TURN n'est pas posé
// (le process de l'application ne le pose que pour ce qu'il lance lui-même,
// chat_tools.go:235), donc un clic rendu « humain » est exact : c'est la personne
// devant l'écran qui a cliqué. Si un agent local déclenche la même route, la
// variable est là et le relais rend « agent ». Je ne choisis pas à la main, et
// c'est voulu : m'attribuer l'autorité humaine serait exactement la faute que
// l'arbitrage du 27/10 interdit.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// peerPilotTimeout : un pilotage n'est pas une génération. C'est lire une liste,
// ou écrire un réglage. pair de peerProbeTimeout : si le voisin ne répond pas en
// 12 s, il n'est pas en train de « réfléchir », il est joignable ou il ne l'est pas.
const peerPilotTimeout = 12 * time.Second

// peerPilotURL : l'URL d'une route de pilotage du voisin, à partir de l'URL de son
// pont telle qu'elle est déclarée. completionsBase fait déjà le travail pénible :
// l'humain colle tantôt http://hote:18190, tantôt …/v1, tantôt …/v1/chat/completions
// (c'est l'adresse du pont qu'on lui demande de copier depuis l'autre machine).
// On repart de la base /v1, donc les trois formes marchent pareil.
func peerPilotURL(p peerEndpoint, route string) string {
	return completionsBase(p.URL) + route
}

// peerPilotDo : la requête vers le voisin, avec les en-têtes du canal.
//
// Les en-têtes ne sont pas décoratifs, et c'est la leçon de peerAsk (L299-321) :
// sans X-Aidros-Canal, le pont d'en face traite ça comme du logiciel tiers.
// X-Aidros-Canal-From dit qui parle ; X-Aidros-Canal-Origin dit QUI a écrit — la seule pièce qui
// permette à l'IA d'en face de savoir si l'ordre vient d'une personne ou d'une
// autre IA, donc si elle est visée.
func peerPilotDo(ctx context.Context, p peerEndpoint, method, route string, corps []byte, origine string) ([]byte, int, error) {
	var corpsLecteur io.Reader
	if corps != nil {
		corpsLecteur = bytes.NewReader(corps)
	}
	req, err := http.NewRequestWithContext(ctx, method, peerPilotURL(p, route), corpsLecteur)
	if err != nil {
		return nil, 0, err
	}
	if corps != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set(canalHeader, "1")
	req.Header.Set(canalFromHeader, canalMachineNom())
	req.Header.Set(canalHopHeader, "1")
	if strings.TrimSpace(origine) == "" {
		origine = canalOrigineIA
	}
	req.Header.Set(canalOriginHeader, origine)
	if p.Key != "" {
		req.Header.Set("Authorization", "Bearer "+p.Key)
	}
	resp, err := peerHTTPClient.Do(req)
	if err != nil {
		return nil, 0, peerDiagnose(p, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("lecture de la réponse du voisin %s : %w", p.Name, err)
	}
	return body, resp.StatusCode, nil
}

// peerPresetsDistants : les presets du voisin, avec le millésime de moteur que son
// poste rend. C'est LA pièce qui manquait le 17/11 : chez la voisine, deux presets
// GYRO-S, l'interface n'affichait que le # NAME du fichier, et elle a sélectionné
// celui dont les chemins pointaient strata-0.1.48-hip. Le bon était en dessous.
// Ici le millésime arrive depuis le poste qui le sait mieux que quiconque.
//
// L'origine est un paramètre, pas une constante : ma première version codait
// « agent » en dur pour cette lecture, ce qui était faux des deux côtés. Depuis le
// menu, c'est la PERSONNE qui clique pour voir la liste de ses moteurs ; et si un
// jour mon agent la consulte par l'outil peer, ce sera l'IA. Le pont d'en face a
// le droit de savoir qui lui demande, y compris pour une lecture.
func peerPresetsDistants(ctx context.Context, p peerEndpoint, origine string) (map[string]any, error) {
	body, code, err := peerPilotDo(ctx, p, http.MethodGet, "/presets", nil, origine)
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, peerStatusError(p, code, body)
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("voisin %s : sa liste de presets n'est pas du JSON attendu", p.Name)
	}
	return out, nil
}

// peerSwitchDistance : basculer le preset du voisin. `id` est l'identifiant rendu
// par peerPresetsDistants, jamais une position — la leçon de handleControlSwitch :
// à distance, la liste a d'autant plus changé qu'on l'a peut-être modifiée.
func peerSwitchDistance(ctx context.Context, p peerEndpoint, id, origine string) (map[string]any, error) {
	corps, _ := json.Marshal(map[string]string{"id": id})
	body, code, err := peerPilotDo(ctx, p, http.MethodPost, "/presets/switch", corps, origine)
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, peerStatusError(p, code, body)
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("voisin %s : sa réponse au basculement n'est pas du JSON attendu", p.Name)
	}
	return out, nil
}

// peerAgentDistance : allumer ou couper l'agent du voisin à distance.
func peerAgentDistance(ctx context.Context, p peerEndpoint, on bool, origine string) (map[string]any, error) {
	corps, _ := json.Marshal(map[string]string{"agent": map[bool]string{true: "on", false: "off"}[on]})
	body, code, err := peerPilotDo(ctx, p, http.MethodPost, "/mode", corps, origine)
	if err != nil {
		return nil, err
	}
	if code != http.StatusOK {
		return nil, peerStatusError(p, code, body)
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("voisin %s : sa réponse sur le mode n'est pas du JSON attendu", p.Name)
	}
	return out, nil
}

// handlePeerPilot : la route locale du pilotage, /api/peers/pilot.
//
// Elle est POST seulement : lire la liste du voisin est une action déclenchée par
// un clic, pas un état que la carte tient à jour en continu. Un GET ferait un appel
// réseau à chaque ouverture du menu, vers une machine qui peut être éteinte.
func handlePeerPilot(w http.ResponseWriter, r *http.Request) {
	if !parallelBuild() {
		sendJSON(w, http.StatusForbidden, map[string]any{
			"ok": false, "error": "canal réservé à la version portable de Cresius",
		})
		return
	}
	if r.Method != http.MethodPost {
		sendJSON(w, http.StatusMethodNotAllowed, map[string]any{"ok": false, "error": "POST seulement"})
		return
	}
	var in struct {
		Name   string `json:"name"`
		Action string `json:"action"` // presets | switch | agent
		ID     string `json:"id"`     // pour switch : l'identifiant du preset
		On     *bool  `json:"on"`     // pour agent : nil = pas demandé
	}
	raw, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err := json.Unmarshal(raw, &in); err != nil {
		sendJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "JSON invalide"})
		return
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		sendJSON(w, http.StatusBadRequest, map[string]any{"ok": false, "error": "aucun voisin désigné"})
		return
	}
	p, ok := peerFind(name)
	if !ok {
		sendJSON(w, http.StatusNotFound, map[string]any{
			"ok": false, "error": "voisin inconnu : " + name + " (le déclarer d'abord, page Réglages, section Pont)",
		})
		return
	}

	// QUI a écrit. PAS peerOrigineCLI() : mesuré le 19/11, cette fonction lit
	// AJEAN_AGENT_TURN dans l'ENVIRONNEMENT DU PROCESS, et un Ajean lancé pendant
	// un tour d'agent l'hérite (chat_tools.go:235 le pose pour tout ce qu'il
	// lance). Sa carte rendrait alors « agent » pour un simple clic de la
	// personne — et l'IA voisine, croyant n'être pas visée, perdrait l'ordre. Le
	// pire des deux sens : un ordre humain qui passe pour une initiative d'IA
	// perd son autorité, alors qu'une initiative d'IA qui passe pour un ordre
	// humain en gagne une qu'elle n'a pas.
	//
	// Ici l'appelant est le NAVIGATEUR de la personne : un agent local ne passe
	// pas par HTTP pour piloter un voisin, il a l'outil peer (peerAsk). La route
	// est donc un acte humain par construction, et ne peut pas être trompée par
	// l'environnement du process qui la sert.
	origine := canalOrigineHumain

	ctx, cancel := context.WithTimeout(context.Background(), peerPilotTimeout)
	defer cancel()

	action := strings.ToLower(strings.TrimSpace(in.Action))
	switch action {
	case "presets":
		out, err := peerPresetsDistants(ctx, p, origine)
		if err != nil {
			sendJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		sendJSON(w, http.StatusOK, out)
		return
	case "switch":
		id := strings.TrimSpace(in.ID)
		if id == "" {
			sendJSON(w, http.StatusBadRequest, map[string]any{
				"ok": false, "error": "aucun preset désigné chez le voisin (l'action presets rend ses id)",
			})
			return
		}
		out, err := peerSwitchDistance(ctx, p, id, origine)
		if err != nil {
			sendJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		// Trace locale : j'ai changé l'état d'AUTRE CHOSE que ma machine. Sans ça,
		// un matin le voisin ne tourne plus sur le même moteur et personne ne sait
		// par qui c'est passé — ni si c'était un ordre ou une initiative.
		fmt.Printf("%s canal : %s a basculé le preset de « %s »\n", green("[ok]"), origine, p.Name)
		sendJSON(w, http.StatusOK, out)
		return
	case "agent":
		if in.On == nil {
			sendJSON(w, http.StatusBadRequest, map[string]any{
				"ok": false, "error": `agent : donner "on" true ou false`,
			})
			return
		}
		out, err := peerAgentDistance(ctx, p, *in.On, origine)
		if err != nil {
			sendJSON(w, http.StatusBadGateway, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		fmt.Printf("%s canal : agent de « %s » %s par %s\n", green("[ok]"), p.Name,
			map[bool]string{true: "allumé", false: "éteint"}[*in.On], origine)
		sendJSON(w, http.StatusOK, out)
		return
	default:
		sendJSON(w, http.StatusBadRequest, map[string]any{
			"ok":    false,
			"error": `action : "presets", "switch" (avec id) ou "agent" (avec on)`,
		})
		return
	}
}
