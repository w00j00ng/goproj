//go:build !windows

package viewer

import (
	"context"
	"os/exec"
)

func editorCommand(ctx context.Context, editor, path string) (*exec.Cmd, error) {
	return exec.CommandContext(ctx, "sh", "-c", editor+" \"$1\"", "tdelta", path), nil
}
