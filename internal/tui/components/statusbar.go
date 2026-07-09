package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/core"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/tui/theme"
)

// StatusBar shows playback information at the bottom.
type StatusBar struct {
	state      core.PlayerState
	view       string
	styles     theme.Styles
	width      int
	position   float64
	duration   float64
	repeatMode string
	shuffle    bool
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

// SetPosition updates the current playback position and duration.
func (s *StatusBar) SetPosition(pos, dur float64) {
	s.position = pos
	s.duration = dur
}

// SetRepeatShuffle updates repeat/shuffle indicators.
func (s *StatusBar) SetRepeatShuffle(repeat string, shuffle bool) {
	s.repeatMode = repeat
	s.shuffle = shuffle
}

// barLayout computes the rendered pieces and the mini-bar geometry.
// ok is false when the bar is not drawn (stopped, or too narrow).
func (s StatusBar) barLayout() (left, right string, barStart, barWidth int, ok bool) {
	if s.state.Current != nil {
		status := "\u25b6"
		if s.state.Status == core.StatusPaused {
			status = "\u23f8"
		}
		posStr := fmtSecs(s.position)
		durStr := fmtSecs(s.duration)
		left = fmt.Sprintf(" %s %s  %s/%s", status, truncate(s.state.Current.Title, 40), posStr, durStr)
	} else {
		left = " \u25a0 Stopped"
	}

	modes := ""
	if s.repeatMode != "" && s.repeatMode != "off" {
		modes += " \u21bb" + s.repeatMode
	}
	if s.shuffle {
		modes += " \u21c4"
	}
	right = fmt.Sprintf("Vol: %d%%%s \u2502 %s ", s.state.Volume, modes, s.view)

	gap := s.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap > 4 && s.state.Current != nil {
		return left, right, lipgloss.Width(left) + 1, gap, true
	}
	return left, right, 0, 0, false
}

// BarBounds returns the column range [start, start+width) of the mini
// progress bar on the status row, or ok=false if no bar is drawn.
func (s StatusBar) BarBounds() (start, width int, ok bool) {
	_, _, start, width, ok = s.barLayout()
	return
}

// View renders the status bar.
func (s StatusBar) View() string {
	if s.width <= 0 {
		return ""
	}
	left, right, _, gap, ok := s.barLayout()
	if ok {
		bar := miniBar(s.position, s.duration, gap)
		return s.styles.StatusBar.Width(s.width).Render(left + " " + bar + " " + right)
	}
	pad := s.width - lipgloss.Width(left) - lipgloss.Width(right)
	if pad < 0 {
		pad = 0
	}
	return s.styles.StatusBar.Width(s.width).Render(left + strings.Repeat(" ", pad) + right)
}

func miniBar(pos, dur float64, width int) string {
	if dur <= 0 || width <= 0 {
		return strings.Repeat("\u2500", width)
	}
	filled := int(pos / dur * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	return strings.Repeat("\u2501", filled) + strings.Repeat("\u2500", width-filled)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "\u2026"
}

func fmtSecs(secs float64) string {
	total := int(secs)
	if total < 0 {
		total = 0
	}
	m := total / 60
	s := total % 60
	return fmt.Sprintf("%d:%02d", m, s)
}
