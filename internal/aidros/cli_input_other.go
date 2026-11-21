//go:build !windows

package aidros

import (
	"io"
	"os"
)

func terminalInput() io.Reader { return os.Stdin }
