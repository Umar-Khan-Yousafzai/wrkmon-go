// Package mediakeys provides platform adapters implementing
// core.MediaRemote, so hardware media keys and OS media-session widgets
// (MPRIS on Linux, SMTC on Windows) can control wrkmon-go's playback.
package mediakeys

import (
	"runtime"

	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/core"
)

// New returns the media-key remote adapter for the current OS, identifying
// itself to the OS media session as appName.
//
// This task wires the port and per-GOOS dispatch scaffold only: every
// branch below returns Noop{}. A later task fills in "linux" with an MPRIS
// adapter and another fills in "windows" with an SMTC adapter; unsupported/
// unknown platforms keep falling back to Noop{}.
func New(appName string) core.MediaRemote {
	switch runtime.GOOS {
	case "linux":
		return Noop{}
	case "windows":
		return Noop{}
	default:
		return Noop{}
	}
}
