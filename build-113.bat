@echo off
rem Construire 1.1.3 : la traduction de la carte du Pont + le nom du projet.
rem Cible : cmd/ajean-memory-test (le build portable, seul a poser BuildFlavor).
rem ⚠ ASCII PUR. ⚠ setlocal (sinon « set X=v & cmd » colle un espace).
setlocal
set GOEXE=C:\Users\julie\go\pkg\mod\golang.org\toolchain@v0.0.1-go1.25.0.windows-amd64\bin\go.exe
set GVER=C:\Users\julie\go\bin\goversioninfo.exe
set GOTOOLCHAIN=local
set CGO_ENABLED=0
cd /d "C:\ProgramData\Ajean v20.1 maj Cresius\data\workspace\atelier-fusion-0201"

echo ===== 0. les .syso AVANT goversioninfo (le millesime Windows y vit) =====
powershell -NoProfile -Command "foreach ($f in 'cmd\ajean-memory-test\resource_windows_amd64.syso','cmd\aidros\resource_windows_amd64.syso') { $i=Get-Item -LiteralPath $f; $b=[IO.File]::ReadAllBytes($f); $u=[Text.Encoding]::Unicode.GetString($b); $n=([regex]::Matches($u,'1\.1\.2')).Count; $q=([regex]::Matches($u,'1\.1\.3')).Count; '   {0,-46} {1,7} o  1.1.2 x{2}  1.1.3 x{3}' -f $f, $i.Length, $n, $q }"

echo ===== 1. regenerer les DEUX .syso de la cible portable =====
echo     (le go:generate de cmd\ajean-memory-test pointe ../aidros/icon.ico)
pushd cmd\ajean-memory-test
"%GVER%" -64 -icon=../aidros/icon.ico -o resource_windows_amd64.syso versioninfo.json 2>&1
echo     CODE_AMD64=%errorlevel%
"%GVER%" -64 -arm -icon=../aidros/icon.ico -o resource_windows_arm64.syso versioninfo.json 2>&1
echo     CODE_ARM64=%errorlevel%
popd
powershell -NoProfile -Command "foreach ($f in 'cmd\ajean-memory-test\resource_windows_amd64.syso','cmd\ajean-memory-test\resource_windows_arm64.syso') { $i=Get-Item -LiteralPath $f; $b=[IO.File]::ReadAllBytes($f); $u=[Text.Encoding]::Unicode.GetString($b); '   {0,-46} {1,7} o  pose {2:dd/HH:mm:ss}  1.1.3 x{3}  Aidros x{4}' -f $f, $i.Length, $i.LastWriteTime, ([regex]::Matches($u,'1\.1\.3')).Count, ([regex]::Matches($u,'Aidros')).Count }"

echo ===== 2. les gardes de version, dans les deux sens =====
echo     (version_flavor_test compare la cible portable a displayVersion ;
echo      version_test compare cmd\aidros a la base nue 1.1.0)
"%GOEXE%" test ./internal/aidros -run TestVersion -v -count=1 2>&1 | findstr /R /C:"=== RUN" /C:"--- " /C:"ok " /C:"FAIL"

echo ===== 3. build =====
"%GOEXE%" build -trimpath -ldflags "-s -w" -o build\aidros-1.1.3.exe .\cmd\ajean-memory-test 2>&1
echo CODE_BUILD=%errorlevel%

echo ===== 4. ce que rend cet exe =====
powershell -NoProfile -Command "$p='build\aidros-1.1.3.exe'; if (-not (Test-Path -LiteralPath $p)) { '   ABSENT'; exit }; $i=Get-Item -LiteralPath $p; $v=$i.VersionInfo; '   taille      : {0} o' -f $i.Length; '   sha256      : ' + (Get-FileHash -LiteralPath $p -Algorithm SHA256).Hash.Substring(0,12); '   FileVersion : ' + $v.FileVersion; '   ProductVer  : ' + $v.ProductVersion; '   ProductNom  : ' + $v.ProductName; '   OrigFichier : ' + $v.OriginalFilename"

echo ===== 5. le correctif survit-il dans les octets =====
echo     (l UI est embarquee brute par go:embed : le JS s y lit tel quel.
echo      Seules les chaines litterales survivent a -s -w, pas les noms Go.)
powershell -NoProfile -Command "$t=[IO.File]::ReadAllText('build\aidros-1.1.3.exe',[Text.Encoding]::GetEncoding(1252)); '   MARQUES DU NEUF (attendu >=1)'; foreach ($m in 'data-i18n=\"bridge.title\"','function tp(key)','name_default','default_name_default','projects.default_name') { '     {0,-28} x{1}' -f $m, ([regex]::Matches($t,[regex]::Escape($m))).Count }; '   MARQUES DE L ANCIEN (attendu 0)'; foreach ($m in 'stitle\">Pont Aidros','id=\"bridge-state\" class=\"api-state\"') { '     {0,-28} x{1}' -f $m, ([regex]::Matches($t,[regex]::Escape($m))).Count }"

echo ===== 6. contre-epreuve : l exe EN SERVICE doit rendre l inverse =====
powershell -NoProfile -Command "foreach ($e in 'C:\ProgramData\Ajean v20.1 maj Cresius\aidros-1.1.2.exe') { $t=[IO.File]::ReadAllText($e,[Text.Encoding]::GetEncoding(1252)); '   ' + (Split-Path $e -Leaf); '   MARQUES DU NEUF (attendu 0 ICI)'; foreach ($m in 'data-i18n=\"bridge.title\"','function tp(key)','name_default','default_name_default','projects.default_name') { '     {0,-28} x{1}' -f $m, ([regex]::Matches($t,[regex]::Escape($m))).Count }; '   MARQUES DE L ANCIEN (attendu >=1 ICI)'; foreach ($m in 'stitle\">Pont Aidros','id=\"bridge-state\" class=\"api-state\"') { '     {0,-28} x{1}' -f $m, ([regex]::Matches($t,[regex]::Escape($m))).Count } }"

echo ===== 7. l instance qui me porte, intacte (lecture seule) =====
powershell -NoProfile -Command "try { $r=Invoke-WebRequest -Uri 'http://127.0.0.1:18090/api/status' -UseBasicParsing -TimeoutSec 15; $j=$r.Content; '   version rendue : ' + ([regex]::Match($j,'\"version\":\"([^\"]+)\"')).Groups[1].Value; '   etat           : ' + ([regex]::Match($j,'\"state\":\"([^\"]+)\"')).Groups[1].Value } catch { '   ERREUR : ' + $_.Exception.Message }"
endlocal
