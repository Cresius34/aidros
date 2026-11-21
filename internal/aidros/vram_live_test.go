package aidros

import (
	"encoding/json"
	"testing"
)

// Ce test ne cree PAS de testHome : il lit la VRAIE config de la machine, celle
// du preset en service, et fait exactement ce que fait l'encart « Appareil »
// (handleBackendDevices -> listDevicesCached -> sampleVramGPUs). But : separer la
// regression GPU de celle du moteur, sans lancer deuxieme instance d'AJEAN (une
// seconde instance se battrait pour la conversation et le lien, voir cmdApp).
//
// Le 15/11, a la fusion 0.20.1 : il appelait prebuiltResolveBin(), une fonction
// du bloc backend_prebuilt.go que l'amont a ABANDONNE en 0.19. Plutot que de
// ramener le bloc, on reproduit ici la resolution de l'amont (web_devices.go
// L313-320, identique a listDevicesCached L464) : engineBin() d'abord — c'est la
// lecture officielle, elle voit le moteur installe meme quand un preset MoE
// laisse BIN vide — puis la cle BIN en repli. dataDir() n'existait pas : la
// fonction de l'amont est dataDirs() (sys_datadir.go:16), elle rend une liste.
func TestVramSurLaVraieMachine(t *testing.T) {
	cfg := ReadConfig()
	t.Logf("dossiers de donnees : %v", dataDirs())
	for _, k := range []string{"ENGINE", "BIN", "MODEL", "HOST", "PORT", "MOE_SOURCE", "MOE_CONFIG"} {
		t.Logf("config %s = %q", k, cfg[k])
	}

	// La resolution du moteur, telle que l'amont la fait.
	bin := engineBin()
	if bin == "" {
		bin = cfg["BIN"]
	}
	t.Logf("moteur resolu : %q (engineBin -> %q)", bin, engineBin())
	t.Logf("isFile(bin) = %v", isFile(bin))
	if bin != "" && isFile(bin) {
		devs := listDevicesCached(bin)
		raw, _ := json.Marshal(devs)
		t.Logf("listDevicesCached -> %d appareil(s) : %s", len(devs), raw)
	}

	gpus := sampleVramGPUs()
	raw, _ := json.Marshal(gpus)
	t.Logf("sampleVramGPUs -> %d carte(s) : %s", len(gpus), raw)
	applyWindowsDedicatedUsage(gpus)
	raw2, _ := json.Marshal(gpus)
	t.Logf("apres applyWindowsDedicatedUsage : %s", raw2)
	if len(gpus) == 0 {
		t.Errorf("aucune carte GPU rendue : l'encart Appareil sera vide")
	}
}
