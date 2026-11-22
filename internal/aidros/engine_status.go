// engine_status.go — dire POURQUOI le moteur s'est arrêté.
//
// llama-server meurt avec un code de sortie qui ne dit rien (1, ou 0xC0000135
// sous Windows) ; la cause est quelques lignes plus haut dans son journal.
// Comme le StatusWriter d'Ollama, on y cherche les lignes d'erreur connues et
// on les remonte telles quelles à l'interface, au lieu d'un « chargement… »
// sans fin ou d'un numéro.
package aidros

import "strings"

// engineErrPrefixes : débuts de ligne qui signalent une erreur du moteur (liste
// reprise d'Ollama, llm/status.go).
var engineErrPrefixes = []string{
	"[err] ", // l'échec d'« aidros serve » lui-même (modèle introuvable, moteur non installable…)
	"error:",
	"CUDA error",
	"ROCm error",
	"cudaMalloc failed",
	"cuda driver library failed",
	"GGML_ASSERT",
	"error loading model",
	"llama_init_from_model:",
	"llama_model_load:",
	"Deepseek2 does not support K-shift",
	"panic:",
	"terminate called",
}

// engineErrContains : motifs d'erreur qui n'arrivent pas en début de ligne.
var engineErrContains = []string{
	"out of memory",
	"failed to allocate",
	"unable to allocate",
	"insufficient memory",
	"ErrorOutOfDeviceMemory",
	"failed to load model",
	"failed to load mmproj",
	"unknown argument",
	"invalid argument",
	"error while loading shared libraries",
	"Library not loaded",
	"couldn't bind",
	"address already in use",
}

// engineErrorFromLog renvoie les dernières lignes d'erreur du journal du moteur
// (au plus trois, les plus récentes en dernier), "" s'il n'y en a pas.
func engineErrorFromLog(log string) string {
	var hits []string
	for _, raw := range strings.Split(strings.ReplaceAll(log, "\r", ""), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		// journald / notre propre préfixe : on cherche dans le message lui-même.
		msg := line
		if i := strings.Index(msg, "]: "); i >= 0 && i < 80 {
			msg = strings.TrimSpace(msg[i+3:])
		}
		// Un nouveau démarrage du moteur efface les erreurs du précédent.
		// Les DEUX préfixes sont admis, et c'est un contrat de LECTURE : le journal
		// survit au remplacement du binaire. Une machine qui a tourné hier sous
		// « [ajean serve] » a ses lignes de relance écrites sous ce nom-là ; n'en
		// reconnaître qu'un seul laisse l'UI afficher l'erreur d'un moteur arrêté
		// depuis, pendant que le nouveau tourne — rien ne se plaint, l'erreur est
		// fausse. Écrire ne connaît que le nouveau.
		if moteurRelanceLigne(msg) {
			hits = hits[:0]
			continue
		}
		if isEngineErrLine(msg) {
			msg = strings.TrimSpace(strings.TrimPrefix(msg, "[err]")) // marque d'« aidros serve », sans intérêt à l'écran
			if len(msg) > 300 {
				msg = msg[:300] + "…"
			}
			if n := len(hits); n == 0 || hits[n-1] != msg {
				hits = append(hits, msg)
			}
		}
	}
	if len(hits) > 3 {
		hits = hits[len(hits)-3:]
	}
	return strings.Join(hits, "\n")
}

// moteurRelanceLigne : la ligne qui annonce un NOUVEAU démarrage du moteur
// (« [aidros serve] … model=… », ou son nom d'hier « [ajean serve] … »). Elle
// porte les deux noms pour la même raison que les en-têtes du pont : le fichier
// où on la cherche a pu être écrit par le binaire d'avant.
func moteurRelanceLigne(msg string) bool {
	if !strings.Contains(msg, "model=") {
		return false
	}
	return strings.HasPrefix(msg, "[aidros serve] ") || strings.HasPrefix(msg, "[ajean serve] ")
}

func isEngineErrLine(msg string) bool {
	low := strings.ToLower(msg)
	for _, p := range engineErrPrefixes {
		lp := strings.ToLower(p)
		if strings.HasPrefix(low, lp) {
			return true
		}
		// llama.cpp préfixe souvent par le niveau et la fonction : « E srv  load_model: error loading model »
		if i := strings.Index(low, lp); i > 0 && i < 60 && (low[i-1] == ' ' || low[i-1] == ':') {
			return true
		}
	}
	for _, c := range engineErrContains {
		if strings.Contains(low, strings.ToLower(c)) {
			return true
		}
	}
	return false
}
