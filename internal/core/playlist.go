package core

import "time"

// Playlist is a named collection of tracks.
type Playlist struct {
	ID        int
	Name      string
	CreatedAt time.Time
	Tracks    []Track
}
