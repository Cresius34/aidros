//go:build !windows

package aidros

import (
	"os"
	"path/filepath"
)

// mediaFace fabrique, HORS WINDOWS, un chemin qui se résout ailleurs que
// lui-même : c'est le cas macOS, celui qui faisait rougir le runner du
// 21/11/2026 (mem_embedding_settings_test.go:103, « lien média hors du projet
// mémoire »).
//
// Sous macOS et Linux, /tmp et /var sont des liens (macOS : /var ->
// /private/var) et créer un lien symbolique n'y demande aucun droit. C'est donc
// le geste EXACT de la machine visée, pas un substitut.
//
// Renvoie une chaine vide si le lien est refusé, avec la raison.
func mediaFace(dir string) (string, string) {
	face := filepath.Join(filepath.Dir(dir), "face-"+filepath.Base(dir))
	if err := os.Symlink(dir, face); err != nil {
		return "", "lien symbolique refusé : " + err.Error()
	}
	return face, ""
}
