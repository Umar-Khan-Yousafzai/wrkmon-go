//go:build darwin

package mpv

// extraMPVArgs returns extra mpv command-line arguments needed on macOS.
// --input-media-keys=yes lets mpv register for the system media keys (and
// the Control Center now-playing widget) itself, since macOS has no
// out-of-process key-grab equivalent to Linux's MPRIS or Windows'
// RegisterHotKey.
func extraMPVArgs() []string {
	return []string{"--input-media-keys=yes"}
}
