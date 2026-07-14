// Package mediakeys provides platform adapters implementing
// core.MediaRemote, so hardware media keys and OS media-session widgets
// (MPRIS on Linux, SMTC on Windows) can control wrkmon-go's playback.
package mediakeys

import (
	"fmt"
	"os"
	"runtime"

	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/core"
)

// New returns the media-key remote adapter for the current OS, identifying
// itself to the OS media session as appName.
//
// On Linux it constructs the MPRIS adapter; if the D-Bus session bus is
// unavailable or the bus name can't be claimed, it logs one warning to stderr
// and falls back to Noop{} so media-key support degrades gracefully without
// blocking startup. On Windows it registers global media-key hotkeys via
// RegisterHotKey; if registration fails outright it likewise logs one
// warning and falls back to Noop{}. Unsupported/unknown platforms keep
// falling back to Noop{}.
func New(appName string) core.MediaRemote {
	switch runtime.GOOS {
	case "linux":
		remote, err := newMPRIS(appName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "wrkmon: media keys disabled (MPRIS unavailable): %v\n", err)
			return Noop{}
		}
		return remote
	case "windows":
		remote, err := newHotkeys()
		if err != nil {
			fmt.Fprintf(os.Stderr, "wrkmon: media keys disabled (hotkeys unavailable): %v\n", err)
			return Noop{}
		}
		return remote
	default:
		return Noop{}
	}
}
