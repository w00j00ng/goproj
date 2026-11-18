//go:build !windows

package cmd

import (
	"os"

	"golang.org/x/term"
)

func supportsColor(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}
