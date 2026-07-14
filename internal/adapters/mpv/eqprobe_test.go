package mpv

import (
	"os/exec"
	"testing"
	"time"

	"github.com/Umar-Khan-Yousafzai/wrkmon-go/core"
)

// TestSetAudioFilter_Integration is a live probe against a real mpv binary:
// it verifies that the superequalizer filter string produced by
// core.EQState.FilterString is syntax mpv 0.37 actually accepts. Unit tests
// can't catch this — mpv rejects a malformed af chain at runtime via the
// IPC error channel, not at compile time.
func TestSetAudioFilter_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("integration")
	}
	if _, err := exec.LookPath("mpv"); err != nil {
		t.Skip("mpv not installed")
	}
	m, err := New("")
	if err != nil {
		t.Fatal(err)
	}
	// Play silence so an af chain can attach: av://lavfi generates a tone
	// without hitting the network.
	if err := m.Play("av://lavfi:anullsrc=d=30"); err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	time.Sleep(1 * time.Second)

	var e core.EQState
	e.Enabled = true
	e.Gains[0] = 6
	if err := m.SetAudioFilter(e.FilterString()); err != nil {
		t.Fatalf("mpv rejected filter %q: %v", e.FilterString(), err)
	}
	if err := m.SetAudioFilter(""); err != nil {
		t.Fatalf("clearing af failed: %v", err)
	}
}
