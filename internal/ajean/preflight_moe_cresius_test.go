package ajean

import (
	"strings"
	"testing"
)

// Le moteur MoE (Strata) est NOTRE moteur : il doit passer le pré-vol de
// `ajean start` / `ajean restart`, là où l'amont 0.20 a mis removedEngine() qui
// répond « ce preset utilise MoE, retiré d'AJEAN 0.20 ».
//
// Mesuré le 2026-11-14 : la fusion 0.20.1 amenait la version de l'amont de
// preflightEngine(), donc le pré-vol échouait AVANT même de tenter le moteur.
// Deux conséquences, la même chaîne :
//   - `ajean serve` ne partait jamais sur serveMoe() : Strata ne s'initialisait pas ;
//   - engineEnsure() n'étant jamais appelé, le moteur livré llama.cpp restait
//     absent, BIN vide — et vulkanVramGPUs, le SEUL repli qui voie une carte AMD
//     sous Windows, rend une liste vide : l'encart « Appareil » ne montrait plus
//     le GPU.
//
// Ce test n'installe pas Strata : il vérifie que le pré-vol ne REJETTE PAS le
// preset pour la raison de l'amont. Un échec de moePreflight pour python/serve
// absents est acceptable et attendu ici — ce n'est pas la faute testée.
func TestPreflightAccepteMoteurMoE(t *testing.T) {
	testHome(t)

	if err := WriteConfig(map[string]string{
		"ENGINE":     "moe",
		"MOE_CONFIG": "strata-iq3_s.json",
	}); err != nil {
		t.Fatal(err)
	}
	err := preflightEngine()
	if err != nil && strings.Contains(err.Error(), "retiré d'AJEAN") {
		t.Errorf("le pré-vol refuse le moteur MoE comme un moteur retiré : %v", err)
	}
	if err != nil && strings.Contains(err.Error(), "BIN non défini") {
		t.Errorf("le pré-vol traite un preset MoE comme un preset llama.cpp : %v", err)
	}

	// La garde de l'amont doit TENIR sur les moteurs vraiment retirés : ExLlama
	// reste refusé, sinon on n'a pas réparé, on a tout ouvert.
	if err := WriteConfig(map[string]string{"ENGINE": "exl3", "EXL3_MODEL": "x"}); err != nil {
		t.Fatal(err)
	}
	if err := preflightEngine(); err == nil || !strings.Contains(err.Error(), "ExLlama") {
		t.Errorf("pré-vol d'un preset ExLlama : %v, attendu le refus « ExLlama »", err)
	}
}

// TestRoutageServeAccepteMoteurMoE exerce le ROUTAGE de serve(), pas le pré-vol :
// c'est là que la fusion 0.20.1 avait remis la version de l'amont, où
// removedEngine() répond « MoE, retiré d'AJEAN 0.20 » avant toute tentative.
//
// Mesuré le 2026-11-15, au moment de poser 0.20.1.2 Cresius : l'arbre de travail
// ne contenait plus l'appel `return serveMoe(cfg)` de backend_serve.go (HEAD, lui,
// le portait à la ligne 255 de 4e9211b). Preuve dans l'octet du binaire bâti depuis
// cet arbre : `serveMoe` et `serveStrataMoe` y comptaient 0 occurrence, quand
// l'exe en service les portait 2 fois chacun — le lien avait éliminé les fonctions
// faute d'appelant. Le test du pré-vol ci-dessus est passé VERT pendant ce temps,
// car il n'appelle pas serve(). Leçon à ne pas perdre : tester la décision qui
// route, pas seulement la garde qui la précède.
//
// Le test n'installe ni ne démarre rien : avec ENGINE=moe et ni MOE_RUNTIME ni
// MOE_CONFIG, serve() part sur serveMoe() puis moePreflight(), qui refuse pour
// MOE_CONFIG absent. Cette erreur-là est la preuve que le routage a pris NOTRE
// branche ; l'erreur de l'amont (« retiré d'AJEAN ») prouverait l'inverse.
// execServer n'atteint donc pas son spawn : pas de moteur de 74 Go, pas de fils.
func TestRoutageServeAccepteMoteurMoE(t *testing.T) {
	testHome(t)

	if err := WriteConfig(map[string]string{"ENGINE": "moe"}); err != nil {
		t.Fatal(err)
	}
	err := cmdServe(nil)
	if err == nil {
		t.Fatal("serve() d'un preset MoE a réussi : on attendait qu'il aille jusqu'à moePreflight et refuse là, pas qu'il revienne propre")
	}
	msg := err.Error()
	if strings.Contains(msg, "retiré d'AJEAN") {
		t.Errorf("serve() route le moteur MoE vers le garde des moteurs retirés : %v", err)
	}
	if !strings.Contains(msg, "MOE_CONFIG") {
		t.Errorf("serve() n'a pas atteint moePreflight (erreur inattendue) : %v", err)
	}

	// La garde de l'amont doit TENIR sur ce que l'amont a vraiment retiré : sinon
	// on n'a pas réparé le routage, on a supprimé la garde pour tout le monde.
	for _, c := range []struct {
		cfg  map[string]string
		veut string
	}{
		{map[string]string{"ENGINE": "exl3", "EXL3_MODEL": "x"}, "ExLlama"},
		{map[string]string{"CLOUD": "modal"}, "GPU Cloud"},
	} {
		if err := WriteConfig(c.cfg); err != nil {
			t.Fatal(err)
		}
		if err := cmdServe(nil); err == nil || !strings.Contains(err.Error(), c.veut) {
			t.Errorf("serve() d'un preset %s : %v, attendu le refus « %s »", c.veut, err, c.veut)
		}
	}
}
