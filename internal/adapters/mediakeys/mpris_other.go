//go:build !linux

package mediakeys

import (
	"errors"

	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/core"
)

// newMPRIS is a stub on non-Linux platforms so the GOOS dispatch in New
// compiles everywhere. It is never reached at runtime (the "linux" case only
// runs when runtime.GOOS == "linux"); the real implementation lives in
// mpris_linux.go.
func newMPRIS(appName string) (core.MediaRemote, error) {
	return nil, errors.New("mpris: unsupported on this platform")
}
