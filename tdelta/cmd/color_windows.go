package cmd

import (
	"os"

	"golang.org/x/sys/windows"
)

func supportsColor(f *os.File) bool {
	var mode uint32
	return windows.GetConsoleMode(windows.Handle(f.Fd()), &mode) == nil && mode&windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING != 0
}
