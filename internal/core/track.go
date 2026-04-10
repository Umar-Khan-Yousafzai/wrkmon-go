package core

import "time"

// Track represents a single audio track (YouTube video).
type Track struct {
	ID        string        // internal UUID
	VideoID   string        // YouTube video ID (11 chars)
	Title     string
	Channel   string
	Duration  time.Duration
	StreamURL string // ephemeral mpv-playable URL from yt-dlp
	AddedAt   time.Time
}
