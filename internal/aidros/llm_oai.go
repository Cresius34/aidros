package aidros

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

// llm_oai.go — le reverse-proxy de la surface compatible OpenAI.
//
// Il logeait sous le relais « Ajean Link » et portait alors deux affaires : le
// front TLS PUBLIC (<machine>.oai.ajean.link, terminé ici, clé privée qui ne
// quitte jamais la machine, certificat Let's Encrypt posé à travers le tunnel)
// et le proxy vers llama-server. Le relais est retiré, le front public avec :
// il n'y a plus de tunnel pour l'héberger, et l'interface refusait l'interrupteur
// tant que le lien n'était pas actif. Ne reste que le proxy.
//
// Il sert aujourd'hui le PONT Aidros↔Aidros (parallel_bridge.go:166), qui prête
// le preset actif par la surface OpenAI. Le pont n'a jamais touché au TLS ni au
// nom de domaine : il emprunte le routage /v1, rien d'autre.
//
// On NE touche PAS à l'en-tête Authorization : le client envoie la vraie clé
// (.api_key), que llama-server valide lui-même (--api-key).

// oaiHandler construit le reverse-proxy vers llama-server, restreint à la surface
// compatible OpenAI.
func oaiHandler() http.Handler {
	llama := &url.URL{Scheme: "http", Host: fmt.Sprintf("127.0.0.1:%d", LLMPort())}
	lp := httputil.NewSingleHostReverseProxy(llama)
	lp.FlushInterval = -1 // streaming SSE des complétions
	base := lp.Director
	lp.Director = func(req *http.Request) {
		base(req)
		// Host local : certains moteurs refusent (403) <machine>.oai.ajean.link, un nom qu'ils ne
		// connaissent pas (protection DNS rebinding).
		req.Host = llama.Host
	}
	lp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, e error) {
		http.Error(w, "llama-server injoignable: "+e.Error(), http.StatusBadGateway)
	}
	ant := anthropicHandler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if isAnthropicPath(p) { // Claude Code : traduit vers le preset actif
			ant.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(p, "/v1") || p == "/health" || p == "/props" || p == "/metrics" || strings.HasPrefix(p, "/slots") {
			lp.ServeHTTP(w, r)
			return
		}
		http.Error(w, "not found (endpoint OpenAI: /v1/*)", http.StatusNotFound)
	})
}
