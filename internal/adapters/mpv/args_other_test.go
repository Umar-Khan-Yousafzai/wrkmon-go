//go:build !darwin

package mpv

import "testing"

// TestExtraMPVArgs_NonDarwin verifies extraMPVArgs contributes nothing on
// non-macOS platforms (media-key passthrough there is handled out-of-process
// by the mediakeys adapters, not by an mpv flag). The darwin-only build tag
// on args_darwin.go makes its own extraMPVArgs untestable from a Linux/CI
// runner, so this is the cross-platform half of the coverage.
func TestExtraMPVArgs_NonDarwin(t *testing.T) {
	got := extraMPVArgs()
	if got != nil {
		t.Fatalf("extraMPVArgs() = %#v, want nil on %s", got, "non-darwin")
	}
}
