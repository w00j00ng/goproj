package viewer

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

func editorCommand(editor, path string) (*exec.Cmd, error) {
	system, err := windows.GetSystemDirectory()
	if err != nil {
		return nil, fmt.Errorf("find cmd.exe: %w", err)
	}
	dir := filepath.Dir(path)
	if strings.HasPrefix(dir, "\\\\") {
		return nil, errors.New("editor temporary directory must be on a local drive on Windows")
	}
	shell := filepath.Join(system, "cmd.exe")
	child := exec.Command(shell)
	child.Dir = dir
	// Only the configured command and the fixed filename go through the cmd parser.
	child.SysProcAttr = &syscall.SysProcAttr{CmdLine: "\"" + shell + "\" /d /v:off /s /c \"" + editor + " change.diff\""}
	return child, nil
}
