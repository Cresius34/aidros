//go:build windows

package aidros

import (
	"syscall"
	"unsafe"
)

// mediaFace fabrique, SOUS WINDOWS, un chemin qui se résout ailleurs que
// lui-même : le cas macOS, où /var est un lien vers /private/var et où la racine
// d'un projet se résout donc hors d'elle-même.
//
// Le lien symbolique Windows exige SeCreateSymbolicLink (ou le mode
// développeur) : sur une machine banale il est refusé et le test sautait sans
// rien prouver. Le NOM COURT 8.3, lui, ne demande aucun droit — il est posé par
// le système de fichiers. Mesuré sur ce poste le 22/11/2026 :
// C:\Users\julie\AppData\Local\Temp\SONDE5~1 se résout en …\sonde597247489,
// donc EvalSymlinks y rend un chemin DIFFÉRENT de celui qu'on lui donne.
//
// Renvoie une chaine vide si aucun moyen n'a marché, avec la raison : l'appelant
// saute alors en la disant, au lieu de sauter en silence.
func mediaFace(dir string) (string, string) {
	k := syscall.NewLazyDLL("kernel32.dll")
	proc := k.NewProc("GetShortPathNameW")
	u, err := syscall.UTF16PtrFromString(dir)
	if err != nil {
		return "", err.Error()
	}
	buf := make([]uint16, 4096)
	r, _, e := proc.Call(uintptr(unsafe.Pointer(u)), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if r == 0 {
		return "", "GetShortPathNameW : " + e.Error()
	}
	court := syscall.UTF16ToString(buf[:int(r)])
	if court == "" || court == dir {
		return "", "le nom court rendu est identique au nom long : rien à résoudre"
	}
	return court, ""
}
