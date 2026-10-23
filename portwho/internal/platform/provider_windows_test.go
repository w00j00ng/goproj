//go:build windows && amd64

package platform

import (
	"errors"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsImageQuery(t *testing.T) {
	for _, tt := range []struct {
		name      string
		grow      int
		failure   error
		badLength int
		wantCalls int
	}{
		{name: "unicode image", wantCalls: 1},
		{name: "buffer growth", grow: 2, wantCalls: 3},
		{name: "bounded growth", grow: 20, wantCalls: 8},
		{name: "access denied", failure: windows.ERROR_ACCESS_DENIED, wantCalls: 1},
		{name: "error 87 remains OS error", failure: windows.ERROR_INVALID_PARAMETER, wantCalls: 1},
		{name: "zero success length", badLength: 1, wantCalls: 1},
		{name: "oversized success length", badLength: 2, wantCalls: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			got, err := queryWindowsImage(func(buf []uint16, n *uint32) error {
				calls++
				if len(buf) > 32768 {
					t.Fatal("buffer exceeded cap")
				}
				if calls <= tt.grow {
					return windows.ERROR_INSUFFICIENT_BUFFER
				}
				if tt.failure != nil {
					return tt.failure
				}
				if tt.badLength == 1 {
					*n = 0
					return nil
				}
				if tt.badLength == 2 {
					*n = uint32(len(buf) + 1)
					return nil
				}
				value, _ := windows.UTF16FromString(`C:\한글 앱\backend.exe`)
				copy(buf, value)
				*n = uint32(len(value) - 1)
				return nil
			})
			if calls != tt.wantCalls {
				t.Fatalf("calls=%d want=%d", calls, tt.wantCalls)
			}
			if tt.grow > 8 || tt.failure != nil || tt.badLength > 0 {
				if err == nil || got != "" {
					t.Fatalf("got=%q err=%v", got, err)
				}
				if tt.failure != nil && !errors.Is(err, tt.failure) {
					t.Fatalf("lost native error: %v", err)
				}
			} else if err != nil || got != `C:\한글 앱\backend.exe` {
				t.Fatalf("got=%q err=%v", got, err)
			}
		})
	}
}
