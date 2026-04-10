package components

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/core"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/tui/theme"
)

// StatusBar shows playback information at the bottom.
type StatusBar struct {
	state  core.PlayerState
	view   string
	styles theme.Styles
	width  int
}

// NewStatusBar creates a status bar.
func NewStatusBar(t theme.Theme) StatusBar {
	return StatusBar{
		styles: t.Styles(),
		view:   "search",
	}
}

// SetState updates the player state.
func (s *StatusBar) SetState(state core.PlayerState) {
	s.state = state
}

// SetView updates the current view name.
func (s *StatusBar) SetView(v string) {
	s.view = v
}

// SetWidth updates the bar width.
func (s *StatusBar) SetWidth(w int) {
	s.width = w
}

// View renders the status bar.
func (s StatusBar) View() string {
	left := ""
	if s.state.Current != nil {
		status := "\u25b6"
		if s.state.Status == core.StatusPaused {
			status = "\u23f8"
		}
		left = fmt.Sprintf(" %s %s", status, s.state.Current.Title)
	} else {
		left = " \u25a0 Stopped"
	}

	right := fmt.Sprintf("Vol: %d%% \u2502 %s ", s.state.Volume, s.view)

	gap := s.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}
	padding := ""
	for i := 0; i < gap; i++ {
		padding += " "
	}

	bar := left + padding + right

	return s.styles.StatusBar.Width(s.width).Render(bar)
}
