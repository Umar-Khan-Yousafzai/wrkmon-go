package components

import (
	"testing"
	"time"

	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/core"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/tui/theme"
	"github.com/charmbracelet/lipgloss"
)

func playingBar(width int) StatusBar {
	s := NewStatusBar(theme.Get("opencode-mono"))
	s.SetWidth(width)
	s.SetState(core.PlayerState{
		Status:  core.StatusPlaying,
		Current: &core.Track{Title: "Test Song", Duration: 100 * time.Second},
		Volume:  50,
	})
	s.SetPosition(10, 100)
	return s
}

func TestBarBoundsPlaying(t *testing.T) {
	s := playingBar(100)
	start, width, ok := s.BarBounds()
	if !ok {
		t.Fatal("expected a bar at width 100 while playing")
	}
	if width <= 4 {
		t.Errorf("bar width = %d, want > 4", width)
	}
	if start <= 0 || start+width >= 100 {
		t.Errorf("bar [%d, %d) out of range for width 100", start, start+width)
	}
	// The bar must sit within the rendered line.
	if lipgloss.Width(s.View()) < start+width {
		t.Error("bar bounds exceed rendered width")
	}
}

func TestBarBoundsStopped(t *testing.T) {
	s := NewStatusBar(theme.Get("opencode-mono"))
	s.SetWidth(100)
	if _, _, ok := s.BarBounds(); ok {
		t.Error("no bar expected when nothing is playing")
	}
}

func TestBarBoundsNarrow(t *testing.T) {
	s := playingBar(30) // too narrow for the mini bar (gap <= 4)
	if _, _, ok := s.BarBounds(); ok {
		t.Error("no bar expected on a narrow status bar")
	}
}
