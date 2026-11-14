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
