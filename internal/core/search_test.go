package core

import (
	"testing"
	"time"
)

func TestSearchResult_ToTrack(t *testing.T) {
	sr := SearchResult{
		VideoID:  "dQw4w9WgXcQ",
		Title:    "Never Gonna Give You Up",
		Channel:  "Rick Astley",
		Duration: 3*time.Minute + 33*time.Second,
	}

	before := time.Now()
	track := sr.ToTrack("test-uuid-123")
	after := time.Now()

	if track.ID != "test-uuid-123" {
		t.Errorf("expected ID 'test-uuid-123', got '%s'", track.ID)
	}
	if track.VideoID != sr.VideoID {
		t.Errorf("expected VideoID '%s', got '%s'", sr.VideoID, track.VideoID)
	}
	if track.Title != sr.Title {
		t.Errorf("expected Title '%s', got '%s'", sr.Title, track.Title)
	}
	if track.Channel != sr.Channel {
		t.Errorf("expected Channel '%s', got '%s'", sr.Channel, track.Channel)
	}
	if track.Duration != sr.Duration {
		t.Errorf("expected Duration %v, got %v", sr.Duration, track.Duration)
	}
	if track.StreamURL != "" {
		t.Errorf("expected empty StreamURL, got '%s'", track.StreamURL)
	}
	if track.AddedAt.Before(before) || track.AddedAt.After(after) {
		t.Errorf("expected AddedAt between %v and %v, got %v", before, after, track.AddedAt)
	}
}
