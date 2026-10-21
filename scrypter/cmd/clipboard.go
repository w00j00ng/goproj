//go:build windows || (cgo && ((linux && !android) || (darwin && !ios)))

package cmd

import (
	"errors"
	"fmt"

	"golang.design/x/clipboard"
)

func writeClipboard(hash []byte) error {
	if err := clipboard.Init(); err != nil {
		return fmt.Errorf("initialize clipboard: %w", err)
	}
	if clipboard.Write(clipboard.FmtText, hash) == nil {
		return errors.New("write clipboard failed")
	}
	return nil
}
