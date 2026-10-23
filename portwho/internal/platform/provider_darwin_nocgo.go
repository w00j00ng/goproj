//go:build darwin && !cgo && (amd64 || arm64)

package platform

import (
	"errors"
	"runtime"

	"github.com/w00j00ng/goproj/portwho/internal/inspect"
)

type darwinNoCGO struct{}

func New() inspect.Provider { return darwinNoCGO{} }

func (darwinNoCGO) Runtime() inspect.Runtime {
	return inspect.Runtime{OS: "darwin", Arch: runtime.GOARCH, Backend: "unsupported", SocketScope: "unavailable", NetworkScope: "unavailable", PIDScope: "unavailable"}
}

func (darwinNoCGO) Collect(families []string) map[string]inspect.FamilyResult {
	results := make(map[string]inspect.FamilyResult, len(families))
	for _, family := range families {
		results[family] = inspect.FamilyResult{Error: &inspect.Diagnostic{Stage: "platform", Code: "backend_unavailable", Message: "The macOS backend requires a build with cgo and the macOS SDK."}}
	}
	return results
}

func (darwinNoCGO) Resolve([]inspect.Socket) (map[string][]uint32, []string) {
	return map[string][]uint32{}, nil
}

func (darwinNoCGO) ImagePath(uint32) (string, error) {
	return "", errors.New("the macOS backend requires cgo")
}
