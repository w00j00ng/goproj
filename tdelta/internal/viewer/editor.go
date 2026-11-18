package viewer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
)

func Configured(explicit string) (string, error) {
	for _, value := range []string{explicit, os.Getenv("TDELTA_EDITOR"), os.Getenv("VISUAL"), os.Getenv("EDITOR")} {
		if strings.TrimSpace(value) != "" {
			return value, nil
		}
	}
	return "", errors.New("no editor configured: use --editor-command, TDELTA_EDITOR, VISUAL or EDITOR")
}

// Open writes a scratch diff and removes it after the configured editor exits.
func Open(editor string, stdin io.Reader, stdout, stderr io.Writer, writeDiff func(io.Writer) error) (err error) {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer stop()

	dir, err := os.MkdirTemp("", "tdelta-")
	if err != nil {
		return fmt.Errorf("create editor directory: %w", err)
	}
	defer func() {
		if cleanup := os.RemoveAll(dir); cleanup != nil {
			err = errors.Join(err, fmt.Errorf("remove editor directory: %w", cleanup))
		}
	}()
	path, err := filepath.Abs(filepath.Join(dir, "change.diff"))
	if err != nil {
		return fmt.Errorf("resolve editor path: %w", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("create editor file: %w", err)
	}
	if err := errors.Join(writeDiff(f), f.Close()); err != nil {
		return fmt.Errorf("write editor file: %w", err)
	}
	child, err := editorCommand(ctx, editor, path)
	if err != nil {
		return err
	}
	child.Stdin, child.Stdout, child.Stderr = stdin, stdout, stderr
	if err := child.Run(); err != nil {
		return fmt.Errorf("run editor: %w", err)
	}
	return nil
}
