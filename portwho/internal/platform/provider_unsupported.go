//go:build (!windows && !linux && !darwin) || (windows && !amd64) || ((linux || darwin) && !amd64 && !arm64)

package platform

import (
	"errors"
	"github.com/w00j00ng/goproj/portwho/internal/inspect"
	"runtime"
)

type unsupportedProvider struct{}

func New() inspect.Provider { return unsupportedProvider{} }
func (unsupportedProvider) Runtime() inspect.Runtime {
	return inspect.Runtime{OS: runtime.GOOS, Arch: runtime.GOARCH, Backend: "unsupported", SocketScope: "unavailable", NetworkScope: "unavailable", PIDScope: "unavailable"}
}
func (unsupportedProvider) Collect(families []string) map[string]inspect.FamilyResult {
	d := &inspect.Diagnostic{Stage: "platform", Code: "unsupported_platform", Message: "unsupported operating system or architecture"}
	r := map[string]inspect.FamilyResult{}
	for _, f := range families {
		r[f] = inspect.FamilyResult{Error: d}
	}
	return r
}
func (unsupportedProvider) Resolve([]inspect.Socket) (map[string][]uint32, []string) { return nil, nil }
func (unsupportedProvider) ImagePath(uint32) (string, error) {
	return "", errors.New("unsupported platform")
}
