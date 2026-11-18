//go:build !windows

package viewer

import "os/exec"

func editorCommand(editor, path string) (*exec.Cmd, error) {
	return exec.Command("sh", "-c", editor+" \"$1\"", "tdelta", path), nil
}
