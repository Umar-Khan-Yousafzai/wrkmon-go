package mpv

import (
	"os/exec"
	"testing"

	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/ports"
)

// Compile-time interface check.
var _ ports.Player = (*MPV)(nil)

func TestNew(t *testing.T) {
	if _, err := exec.LookPath("mpv"); err != nil {
		t.Skip("mpv not on PATH")
	}
	m, err := New("")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if m == nil {
		t.Fatal("New returned nil")
	}
	if m.IsRunning() {
		t.Error("should not be running before Play")
	}
	if m.BinPath() == "" {
		t.Error("BinPath should be populated")
	}
}

func TestNew_ConfigNotFound(t *testing.T) {
	_, err := New("/nonexistent/path/to/mpv-xyz")
	if err == nil {
		t.Error("expected error for bogus config path")
	}
}

func TestPlay_NoMpv(t *testing.T) {
	if _, err := exec.LookPath("mpv"); err == nil {
		t.Skip("mpv is available, skipping no-mpv test")
	}
	_, err := New("")
	if err == nil {
		t.Error("expected error when mpv not found")
	}
}

func TestStopBeforePlay(t *testing.T) {
	m := &MPV{}
	if err := m.Stop(); err != nil {
		t.Errorf("Stop on fresh instance: %v", err)
	}
}

func TestCloseBeforePlay(t *testing.T) {
	m := &MPV{}
	if err := m.Close(); err != nil {
		t.Errorf("Close on fresh instance: %v", err)
	}
}

func TestPauseNotConnected(t *testing.T) {
	m := &MPV{}
	if err := m.Pause(); err == nil {
		t.Error("expected error when Pause called without connection")
	}
}

func TestResumeNotConnected(t *testing.T) {
	m := &MPV{}
	if err := m.Resume(); err == nil {
		t.Error("expected error when Resume called without connection")
	}
}

func TestSeekNotConnected(t *testing.T) {
	m := &MPV{}
	if err := m.Seek(10); err == nil {
		t.Error("expected error when Seek called without connection")
	}
}

func TestSetVolumeNotConnected(t *testing.T) {
	m := &MPV{}
	if err := m.SetVolume(50); err == nil {
		t.Error("expected error when SetVolume called without connection")
	}
}

func TestGetPositionNotConnected(t *testing.T) {
	m := &MPV{}
	if _, err := m.GetPosition(); err == nil {
		t.Error("expected error when GetPosition called without connection")
	}
}
