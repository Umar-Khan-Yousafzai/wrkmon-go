package tui

import (
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/core"
)

// SearchResultMsg carries search results back to the TUI.
type SearchResultMsg struct {
	Results []core.SearchResult
	Query   string
	Err     error
}

// StreamURLMsg carries a resolved stream URL.
type StreamURLMsg struct {
	Track core.Track
	URL   string
	Err   error
}

// PlaybackStartedMsg signals playback has started.
type PlaybackStartedMsg struct {
	Track core.Track
}

// PlaybackStoppedMsg signals playback has ended.
type PlaybackStoppedMsg struct{}

// PlaybackErrorMsg signals a playback error.
type PlaybackErrorMsg struct {
	Err error
}

// PositionUpdateMsg carries current playback position and duration.
type PositionUpdateMsg struct {
	Position float64
	Duration float64
}

// HistoryLoadedMsg carries loaded history entries.
type HistoryLoadedMsg struct {
	Entries []core.HistoryEntry
	Err     error
}

// ToastMsg displays a temporary notification.
type ToastMsg struct {
	Text  string
	IsErr bool
}

// ViewChangeMsg switches the active view/context.
type ViewChangeMsg struct {
	View string // "search", "queue", "history", "nowplaying"
}
