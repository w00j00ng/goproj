//go:build !windows && (!cgo || (!linux && !darwin) || android || ios)

package cmd

import "errors"

func writeClipboard(_ []byte) error {
	return errors.New("clipboard is unavailable in this build; use Windows or a supported desktop build with CGO enabled")
}
