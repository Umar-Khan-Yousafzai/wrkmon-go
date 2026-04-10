package core

import "time"

// SearchResult represents a single YouTube search result.
type SearchResult struct {
	VideoID  string
	Title    string
	Channel  string
	Duration time.Duration
}

// ToTrack converts a SearchResult to a Track for queueing.
func (r SearchResult) ToTrack(id string) Track {
	return Track{
		ID:       id,
		VideoID:  r.VideoID,
		Title:    r.Title,
		Channel:  r.Channel,
		Duration: r.Duration,
		AddedAt:  time.Now(),
	}
}
