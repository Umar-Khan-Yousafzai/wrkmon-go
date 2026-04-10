package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/core"
)

func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestNewSQLiteStore_CreatesDB(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "sub", "dir", "test.db")
	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer s.Close()

	// Verify the database is usable by running a simple query.
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM history").Scan(&count); err != nil {
		t.Fatalf("query after create: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 rows, got %d", count)
	}
}

func makeTrack(id, videoID, title, channel string, dur time.Duration) core.Track {
	return core.Track{
		ID:       id,
		VideoID:  videoID,
		Title:    title,
		Channel:  channel,
		Duration: dur,
		AddedAt:  time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC),
	}
}

func makeEntry(id string, track core.Track, playedAt time.Time) core.HistoryEntry {
	return core.HistoryEntry{
		ID:       id,
		Track:    track,
		PlayedAt: playedAt,
	}
}

func TestSaveAndGetHistory_RoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	track := makeTrack("t1", "dQw4w9WgXcQ", "Never Gonna Give You Up", "Rick Astley", 3*time.Minute+33*time.Second)
	entry := makeEntry("h1", track, time.Date(2026, 4, 10, 14, 30, 0, 0, time.UTC))

	if err := s.SaveHistory(ctx, entry); err != nil {
		t.Fatalf("SaveHistory: %v", err)
	}

	got, err := s.GetHistory(ctx, 10, 0)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(got))
	}

	g := got[0]
	if g.ID != entry.ID {
		t.Errorf("ID: got %q, want %q", g.ID, entry.ID)
	}
	if g.Track.VideoID != track.VideoID {
		t.Errorf("VideoID: got %q, want %q", g.Track.VideoID, track.VideoID)
	}
	if g.Track.Title != track.Title {
		t.Errorf("Title: got %q, want %q", g.Track.Title, track.Title)
	}
	if g.Track.Channel != track.Channel {
		t.Errorf("Channel: got %q, want %q", g.Track.Channel, track.Channel)
	}
	if g.Track.Duration != track.Duration {
		t.Errorf("Duration: got %v, want %v", g.Track.Duration, track.Duration)
	}
	if !g.PlayedAt.Equal(entry.PlayedAt) {
		t.Errorf("PlayedAt: got %v, want %v", g.PlayedAt, entry.PlayedAt)
	}
}

func TestGetHistory_LimitAndOffset(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	base := time.Date(2026, 4, 10, 10, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		track := makeTrack("t"+string(rune('0'+i)), "vid"+string(rune('0'+i)), "Song "+string(rune('A'+i)), "Artist", time.Minute)
		entry := makeEntry("h"+string(rune('0'+i)), track, base.Add(time.Duration(i)*time.Hour))
		if err := s.SaveHistory(ctx, entry); err != nil {
			t.Fatalf("SaveHistory[%d]: %v", i, err)
		}
	}

	tests := []struct {
		name     string
		limit    int
		offset   int
		wantLen  int
		wantFirst string // expected title of first result (most recent)
	}{
		{"all", 10, 0, 5, "Song E"},
		{"limit 2", 2, 0, 2, "Song E"},
		{"offset 2", 10, 2, 3, "Song C"},
		{"limit 1 offset 4", 1, 4, 1, "Song A"},
		{"offset past end", 10, 10, 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := s.GetHistory(ctx, tt.limit, tt.offset)
			if err != nil {
				t.Fatalf("GetHistory: %v", err)
			}
			if len(got) != tt.wantLen {
				t.Fatalf("len: got %d, want %d", len(got), tt.wantLen)
			}
			if tt.wantLen > 0 && got[0].Track.Title != tt.wantFirst {
				t.Errorf("first title: got %q, want %q", got[0].Track.Title, tt.wantFirst)
			}
		})
	}
}

func TestSearchHistory_FindsByTitleSubstring(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	entries := []struct {
		id    string
		title string
	}{
		{"h1", "Bohemian Rhapsody"},
		{"h2", "Stairway to Heaven"},
		{"h3", "Hotel California"},
		{"h4", "bohemian nights"}, // lowercase to test case insensitivity
	}

	base := time.Date(2026, 4, 10, 10, 0, 0, 0, time.UTC)
	for i, e := range entries {
		track := makeTrack("t"+e.id, "vid"+e.id, e.title, "Artist", time.Minute)
		entry := makeEntry(e.id, track, base.Add(time.Duration(i)*time.Hour))
		if err := s.SaveHistory(ctx, entry); err != nil {
			t.Fatalf("SaveHistory: %v", err)
		}
	}

	tests := []struct {
		query   string
		wantLen int
	}{
		{"bohemian", 2}, // case-insensitive, matches both
		{"heaven", 1},
		{"xyz", 0},
		{"California", 1},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got, err := s.SearchHistory(ctx, tt.query, 10)
			if err != nil {
				t.Fatalf("SearchHistory: %v", err)
			}
			if len(got) != tt.wantLen {
				t.Errorf("query %q: got %d results, want %d", tt.query, len(got), tt.wantLen)
			}
		})
	}
}

func TestSaveAndLoadQueue_RoundTrip(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	tracks := []core.Track{
		makeTrack("t1", "vid1", "Song A", "Artist 1", 3*time.Minute),
		makeTrack("t2", "vid2", "Song B", "Artist 2", 4*time.Minute+30*time.Second),
		makeTrack("t3", "vid3", "Song C", "Artist 3", 2*time.Minute+15*time.Second),
	}
	tracks[0].StreamURL = "https://example.com/stream1"
	tracks[1].StreamURL = "https://example.com/stream2"

	cursor := 1
	if err := s.SaveQueue(ctx, tracks, cursor); err != nil {
		t.Fatalf("SaveQueue: %v", err)
	}

	gotTracks, gotCursor, err := s.LoadQueue(ctx)
	if err != nil {
		t.Fatalf("LoadQueue: %v", err)
	}

	if gotCursor != cursor {
		t.Errorf("cursor: got %d, want %d", gotCursor, cursor)
	}
	if len(gotTracks) != len(tracks) {
		t.Fatalf("tracks len: got %d, want %d", len(gotTracks), len(tracks))
	}

	for i, want := range tracks {
		got := gotTracks[i]
		if got.ID != want.ID {
			t.Errorf("[%d] ID: got %q, want %q", i, got.ID, want.ID)
		}
		if got.VideoID != want.VideoID {
			t.Errorf("[%d] VideoID: got %q, want %q", i, got.VideoID, want.VideoID)
		}
		if got.Title != want.Title {
			t.Errorf("[%d] Title: got %q, want %q", i, got.Title, want.Title)
		}
		if got.Channel != want.Channel {
			t.Errorf("[%d] Channel: got %q, want %q", i, got.Channel, want.Channel)
		}
		if got.Duration != want.Duration {
			t.Errorf("[%d] Duration: got %v, want %v", i, got.Duration, want.Duration)
		}
		if got.StreamURL != want.StreamURL {
			t.Errorf("[%d] StreamURL: got %q, want %q", i, got.StreamURL, want.StreamURL)
		}
		if !got.AddedAt.Equal(want.AddedAt) {
			t.Errorf("[%d] AddedAt: got %v, want %v", i, got.AddedAt, want.AddedAt)
		}
	}
}

func TestSaveQueue_ReplacesPreviousQueue(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// Save first queue.
	tracks1 := []core.Track{
		makeTrack("t1", "vid1", "Song A", "Artist 1", time.Minute),
		makeTrack("t2", "vid2", "Song B", "Artist 2", time.Minute),
	}
	if err := s.SaveQueue(ctx, tracks1, 0); err != nil {
		t.Fatalf("SaveQueue 1: %v", err)
	}

	// Save second queue — should replace the first.
	tracks2 := []core.Track{
		makeTrack("t3", "vid3", "Song C", "Artist 3", time.Minute),
	}
	if err := s.SaveQueue(ctx, tracks2, 0); err != nil {
		t.Fatalf("SaveQueue 2: %v", err)
	}

	got, cursor, err := s.LoadQueue(ctx)
	if err != nil {
		t.Fatalf("LoadQueue: %v", err)
	}
	if cursor != 0 {
		t.Errorf("cursor: got %d, want 0", cursor)
	}
	if len(got) != 1 {
		t.Fatalf("tracks len: got %d, want 1", len(got))
	}
	if got[0].Title != "Song C" {
		t.Errorf("title: got %q, want %q", got[0].Title, "Song C")
	}
}

func TestLoadQueue_EmptyReturnsNegativeCursor(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	tracks, cursor, err := s.LoadQueue(ctx)
	if err != nil {
		t.Fatalf("LoadQueue: %v", err)
	}
	if len(tracks) != 0 {
		t.Errorf("tracks len: got %d, want 0", len(tracks))
	}
	if cursor != -1 {
		t.Errorf("cursor: got %d, want -1", cursor)
	}
}
