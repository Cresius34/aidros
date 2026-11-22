package aidros

// vram_live_test.go — l'encart « Appareil » sur la VRAIE machine.
//
// Ce test ne cree PAS de testHome : il lit la VRAIE config de la machine, celle
// du preset en service, et fait exactement ce que fait l'encart « Appareil »
// (handleBackendDevices -> listDevicesCached -> sampleVramGPUs). But : separer la
// regression GPU de celle du moteur, sans lancer deuxieme instance d'Aidros (une
// seconde instance se battrait pour la conversation et le lien, voir cmdApp).
//
// Le 15/11, a la fusion 0.20.1 : il appelait prebuiltResolveBin(), une fonction
// du bloc backend_prebuilt.go que l'amont a ABANDONNE en 0.19. Plutot que de
// ramener le bloc, on reproduit ici la resolution de l'amont (web_devices.go
// L313-320, identique a listDevicesCached L464) : engineBin() d'abord — c'est la
// lecture officielle, elle voit le moteur installe meme quand un preset MoE
// laisse BIN vide — puis la cle BIN en repli. dataDir() n'existait pas : la
// fonction de l'amont est dataDirs() (sys_datadir.go:16), elle rend une liste.
//
// LA GARDE DU 22/11, et pourquoi. Depuis que la CI existe (le fork aieul n'avait
// aucun run d'Actions), ce test rougit sur le runner ubuntu : « aucune carte GPU
// rendue ». Ce n'est pas une regression, c'est une absence de materiel — mais un
// saut qui ne dit pas ce qu'il a verifie vaudrait exactement ce qu'il doit
// eviter : masquer un jour un vrai silence de l'echantillonneur. Donc on ne
// saute QUE si AUCUN des quatre chemins de sampleVramGPUs (web_api.go:250-307)
// n'est praticable sur cette machine, et le saut enumere ce qui a ete teste :
//
//	appleGPUs        → runtime.GOOS=darwin + GOARCH=arm64 + ioreg exploitable
//	nvidia-smi       → hasTool, puis l'execution
//	amd-smi/rocm-smi → amdSmiAllowed() (JAMAIS sous Windows) + hasTool
//	vulkanVramGPUs   → engineBin() non vide, puis --list-devices du moteur
//
// Des qu'un de ces chemins est praticable, le test garde sa morsure : s'il ne
// rend aucune carte alors que l'outil est la, l'encart Appareil sera vide chez
// l'humain, et c'est bien ce qu'on veut savoir.

import (
	"encoding/json"
	"runtime"
	"testing"
)

// cheminsGPUPraticables rend la liste des chemins d'echantillonnage que CETTE
// machine peut suivre, vide si aucun. Vide => le test saute, en le disant.
func cheminsGPUPraticables() []string {
	out := []string{}
	if runtime.GOOS == "darwin" && runtime.GOARCH == "arm64" {
		out = append(out, "ioreg (Apple Silicon)")
	}
	if hasTool("nvidia-smi") {
		out = append(out, "nvidia-smi")
	}
	if amdSmiAllowed() {
		if hasTool("amd-smi") {
			out = append(out, "amd-smi")
		}
		if hasTool("rocm-smi") {
			out = append(out, "rocm-smi")
		}
	}
	if bin := engineBin(); bin != "" && isFile(bin) {
		out = append(out, "vulkan via le moteur "+bin)
	}
	return out
}

func TestVramSurLaVraieMachine(t *testing.T) {
	praticables := cheminsGPUPraticables()
	if gpuTelemetryOff() {
		t.Skip("AJEAN_GPU_TELEMETRY=off : la telemetrie est coupee a la source, rien à échantillonner ici")
	}
	if len(praticables) == 0 {
		t.Skip("aucun chemin d'échantillonnage GPU praticable sur cette machine : " +
			"ni ioreg Apple, ni nvidia-smi, ni amd-smi/rocm-smi (hors Windows), ni moteur pour --list-devices")
	}
	t.Logf("chemins GPU praticables ici : %v", praticables)

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
		t.Errorf("aucune carte GPU rendue alors que ces chemins sont praticables : %v — l'encart Appareil sera vide", praticables)
	}
}
