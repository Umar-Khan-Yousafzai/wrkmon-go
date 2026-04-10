package mpv

import (
	"os/exec"
	"testing"

	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/ports"
)

// Compile-time interface check.
var _ ports.Player = (*MPV)(nil)

func TestNew(t *testing.T) {
	m := New()
	if m == nil {
		t.Fatal("New returned nil")
	}
	if m.IsRunning() {
		t.Error("should not be running before Play")
	}
}

func TestPlay_NoMpv(t *testing.T) {
	if _, err := exec.LookPath("mpv"); err == nil {
		t.Skip("mpv is available, skipping no-mpv test")
	}
	m := New()
	err := m.Play("https://example.com/audio.mp3")
	if err == nil {
		t.Error("expected error when mpv not found")
	}
}

func TestStopBeforePlay(t *testing.T) {
	m := New()
	// Stop on a fresh instance should be a no-op, not a panic.
	if err := m.Stop(); err != nil {
		t.Errorf("Stop on fresh instance: %v", err)
	}
}

func TestCloseBeforePlay(t *testing.T) {
	m := New()
	// Close on a fresh instance should be a no-op, not a panic.
	if err := m.Close(); err != nil {
		t.Errorf("Close on fresh instance: %v", err)
	}
}

func TestPauseNotConnected(t *testing.T) {
	m := New()
	err := m.Pause()
	if err == nil {
		t.Error("expected error when Pause called without connection")
	}
}

func TestResumeNotConnected(t *testing.T) {
	m := New()
	err := m.Resume()
	if err == nil {
		t.Error("expected error when Resume called without connection")
	}
}

func TestSeekNotConnected(t *testing.T) {
	m := New()
	err := m.Seek(10)
	if err == nil {
		t.Error("expected error when Seek called without connection")
	}
}

func TestSetVolumeNotConnected(t *testing.T) {
	m := New()
	err := m.SetVolume(50)
	if err == nil {
		t.Error("expected error when SetVolume called without connection")
	}
}

func TestGetPositionNotConnected(t *testing.T) {
	m := New()
	_, err := m.GetPosition()
	if err == nil {
		t.Error("expected error when GetPosition called without connection")
	}
}
