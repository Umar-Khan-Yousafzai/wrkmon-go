package mediakeys

import (
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/core"
	"github.com/godbus/dbus/v5"
)

// mprisTrackID is the fixed MPRIS mpris:trackid object path for the single
// logical "current track" wrkmon-go exposes. wrkmon-go has no track list, so
// one stable trackid is sufficient and satisfies clients that require the key.
const mprisTrackID dbus.ObjectPath = "/org/wrkmon/track/1"

// mprisMetadata maps a core.NowPlaying snapshot to an MPRIS "Metadata"
// (a{sv}) value. Keys follow the MPRIS v2 metadata spec:
//   - mpris:trackid  (o)  stable object path for the current track
//   - mpris:length   (x)  track length in MICROSECONDS
//   - xesam:title    (s)  track title
//   - xesam:artist   (as) list of artists
//
// A blank artist emits an empty list ([]string{}), never [""]: a single blank
// entry reads as an artist literally named "" in playerctl/desktop widgets.
//
// This helper is pure (no D-Bus connection) so it is unit-testable on any
// platform and has no build constraint.
func mprisMetadata(np core.NowPlaying) map[string]dbus.Variant {
	artists := []string{}
	if np.Artist != "" {
		artists = []string{np.Artist}
	}
	return map[string]dbus.Variant{
		"mpris:trackid": dbus.MakeVariant(mprisTrackID),
		"mpris:length":  dbus.MakeVariant(np.Duration.Microseconds()),
		"xesam:title":   dbus.MakeVariant(np.Title),
		"xesam:artist":  dbus.MakeVariant(artists),
	}
}

// playbackStatus maps a core.NowPlaying snapshot to an MPRIS
// "PlaybackStatus" value:
//   - Playing            -> "Playing"
//   - not Playing, title -> "Paused"  (track loaded but paused)
//   - not Playing, empty -> "Stopped" (nothing loaded)
func playbackStatus(np core.NowPlaying) string {
	if np.Playing {
		return "Playing"
	}
	if np.Title != "" {
		return "Paused"
	}
	return "Stopped"
}
