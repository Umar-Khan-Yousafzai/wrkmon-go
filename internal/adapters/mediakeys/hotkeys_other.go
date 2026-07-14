//go:build !windows

package mediakeys

import (
	"errors"

	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/core"
)

// newHotkeys is a stub on non-Windows platforms so the GOOS dispatch in New
// compiles everywhere. It is never reached at runtime (the "windows" case
// only runs when runtime.GOOS == "windows"); the real implementation lives
// in hotkeys_windows.go.
func newHotkeys() (core.MediaRemote, error) {
	return nil, errors.New("hotkeys: unsupported on this platform")
}
