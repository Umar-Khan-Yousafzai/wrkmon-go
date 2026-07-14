//go:build !darwin

package mpv

// extraMPVArgs returns extra mpv command-line arguments for platforms with
// no special media-key passthrough flag: on Linux media keys are handled
// out-of-process by the mediakeys MPRIS adapter, and on Windows by the
// mediakeys RegisterHotKey adapter, so mpv itself needs nothing extra.
func extraMPVArgs() []string {
	return nil
}
