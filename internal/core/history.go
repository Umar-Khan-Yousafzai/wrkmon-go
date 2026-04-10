package core

import "time"

// HistoryEntry records a played track.
type HistoryEntry struct {
	ID       string // UUID
	Track    Track
	PlayedAt time.Time
}
