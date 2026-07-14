//go:build linux

package mediakeys

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/core"
	"github.com/godbus/dbus/v5"
)

// TestMPRISRealBus exercises the adapter against the live session bus: a
// separate client connection calls PlayPause on our exported object and we
// assert the mapped RemoteCommand arrives on Commands() within 2s. If
// playerctl is installed we also assert it reports the published status.
//
// Guarded: skipped under -short and when no session bus is available.
func TestMPRISRealBus(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-bus MPRIS test in -short mode")
	}
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		t.Skip("no DBUS_SESSION_BUS_ADDRESS; skipping real-bus MPRIS test")
	}

	remote, err := newMPRIS("wrkmon-go")
	if err != nil {
		t.Fatalf("newMPRIS: %v", err)
	}
	t.Cleanup(func() { _ = remote.Close() })

	// Publish a playing snapshot so both PropertiesChanged and playerctl
	// observe real metadata/status.
	remote.Publish(core.NowPlaying{
		Title:    "Test Song",
		Artist:   "Tester",
		Duration: 3 * time.Minute,
		Position: 5 * time.Second,
		Playing:  true,
	})

	// Independent client connection (not the shared server connection).
	client, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatalf("client ConnectSessionBus: %v", err)
	}
	defer client.Close()

	obj := client.Object(mprisBusName, mprisPath)
	call := obj.Call(mprisPlayerIface+".PlayPause", 0)
	if call.Err != nil {
		t.Fatalf("client PlayPause call: %v", call.Err)
	}

	select {
	case cmd := <-remote.Commands():
		if cmd != core.RemotePlayPause {
			t.Fatalf("got command %v, want RemotePlayPause", cmd)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for PlayPause command on Commands()")
	}

	// Also verify Next maps correctly over the bus.
	if call := obj.Call(mprisPlayerIface+".Next", 0); call.Err != nil {
		t.Fatalf("client Next call: %v", call.Err)
	}
	select {
	case cmd := <-remote.Commands():
		if cmd != core.RemoteNext {
			t.Fatalf("got command %v, want RemoteNext", cmd)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Next command on Commands()")
	}

	// playerctl cross-check (only if installed).
	if _, err := exec.LookPath("playerctl"); err != nil {
		t.Log("playerctl not installed; skipping playerctl status assertion")
		return
	}
	out, err := exec.Command("playerctl", "--player=wrkmon_go", "status").CombinedOutput()
	if err != nil {
		t.Fatalf("playerctl status: %v (output: %s)", err, strings.TrimSpace(string(out)))
	}
	if got := strings.TrimSpace(string(out)); got != "Playing" {
		t.Fatalf("playerctl status = %q, want %q", got, "Playing")
	}

	// playerctl metadata title cross-check.
	mout, err := exec.Command("playerctl", "--player=wrkmon_go", "metadata", "xesam:title").CombinedOutput()
	if err != nil {
		t.Fatalf("playerctl metadata: %v (output: %s)", err, strings.TrimSpace(string(mout)))
	}
	if got := strings.TrimSpace(string(mout)); got != "Test Song" {
		t.Fatalf("playerctl xesam:title = %q, want %q", got, "Test Song")
	}

	// Close must release cleanly and be idempotent.
	if err := remote.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := remote.Close(); err != nil {
		t.Fatalf("second Close (must be idempotent): %v", err)
	}
}

// TestMPRISPushDropsWhenFull verifies push never blocks: once the buffered
// channel is full, further commands are dropped rather than stalling the
// (D-Bus handler) caller. No bus connection is required.
func TestMPRISPushDropsWhenFull(t *testing.T) {
	r := &mprisRemote{commands: make(chan core.RemoteCommand, 8)}

	// Fill the buffer to capacity, then push extra; none of these must block.
	done := make(chan struct{})
	go func() {
		for i := 0; i < 8; i++ {
			r.push(core.RemotePlayPause)
		}
		// These would block if push blocked on a full buffer.
		r.push(core.RemoteNext)
		r.push(core.RemoteStop)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("push blocked on a full buffer; must drop instead")
	}

	if got := len(r.commands); got != 8 {
		t.Fatalf("buffered commands = %d, want 8 (extras dropped)", got)
	}
}

// TestMPRISCloseIdempotentNoConn verifies Close is a safe no-op when there is
// no live connection and when called repeatedly.
func TestMPRISCloseIdempotentNoConn(t *testing.T) {
	r := &mprisRemote{commands: make(chan core.RemoteCommand, 8)}
	if err := r.Close(); err != nil {
		t.Fatalf("Close with nil conn: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	// A publish after close must be a no-op and must not panic.
	r.Publish(core.NowPlaying{Title: "x", Playing: true})
}

// TestMPRISPublishAfterBusDropDoesNotPanic covers Fix 1: godbus SetMust panics
// when the PropertiesChanged emit fails, which happens when the session bus
// dies underneath a live adapter (dbus restart, suspend/resume). We simulate
// that by closing the underlying connection WITHOUT going through Close (which
// would set the closed guard and make Publish an early-return no-op), forcing
// Publish through the property-set path with a dead connection — the exact
// path that used to crash the TUI goroutine. It must now log and continue.
//
// Guarded: needs a live session bus, skipped under -short.
func TestMPRISPublishAfterBusDropDoesNotPanic(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping real-bus MPRIS test in -short mode")
	}
	if os.Getenv("DBUS_SESSION_BUS_ADDRESS") == "" {
		t.Skip("no DBUS_SESSION_BUS_ADDRESS; skipping real-bus MPRIS test")
	}

	remote, err := newMPRIS("wrkmon-go")
	if err != nil {
		t.Fatalf("newMPRIS: %v", err)
	}
	r := remote.(*mprisRemote)
	t.Cleanup(func() { _ = r.Close() })

	// Establish a "last" state so the next publish is a changed one that must
	// emit PlaybackStatus/Metadata (the signals that fail on a dead bus).
	r.Publish(core.NowPlaying{Title: "Before Drop", Playing: false})

	// Kill the connection out from under the adapter (closed guard stays false).
	_ = r.conn.Close()

	// These both drive a changed publish through the emit path; neither may panic.
	r.Publish(core.NowPlaying{Title: "After Drop", Playing: true, Position: 1 * time.Second})
	r.Publish(core.NowPlaying{Title: "After Drop 2", Playing: false, Position: 3 * time.Second})
}
