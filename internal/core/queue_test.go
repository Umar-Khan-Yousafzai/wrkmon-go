package core

import (
	"testing"
	"time"
)

func makeTrack(id, title string) Track {
	return Track{
		ID:      id,
		VideoID: "dQw4w9WgXcQ",
		Title:   title,
		Channel: "TestChannel",
		Duration: 3 * time.Minute,
		AddedAt: time.Now(),
	}
}

func TestQueue(t *testing.T) {
	tests := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{
			name: "NewQueue starts empty with cursor -1",
			fn: func(t *testing.T) {
				q := NewQueue()
				if q.Len() != 0 {
					t.Errorf("expected Len 0, got %d", q.Len())
				}
				if !q.IsEmpty() {
					t.Error("expected IsEmpty true")
				}
				if q.cursor != -1 {
					t.Errorf("expected cursor -1, got %d", q.cursor)
				}
			},
		},
		{
			name: "Add increases length",
			fn: func(t *testing.T) {
				q := NewQueue()
				q.Add(makeTrack("1", "Track 1"))
				if q.Len() != 1 {
					t.Errorf("expected Len 1, got %d", q.Len())
				}
				q.Add(makeTrack("2", "Track 2"))
				if q.Len() != 2 {
					t.Errorf("expected Len 2, got %d", q.Len())
				}
				if q.IsEmpty() {
					t.Error("expected IsEmpty false after adding")
				}
			},
		},
		{
			name: "Current returns false on empty queue",
			fn: func(t *testing.T) {
				q := NewQueue()
				_, ok := q.Current()
				if ok {
					t.Error("expected Current to return false on empty queue")
				}
			},
		},
		{
			name: "Add then Current returns first track",
			fn: func(t *testing.T) {
				q := NewQueue()
				q.Add(makeTrack("1", "First"))
				track, ok := q.Current()
				if !ok {
					t.Fatal("expected Current to return true after Add")
				}
				if track.ID != "1" {
					t.Errorf("expected track ID '1', got '%s'", track.ID)
				}
				if track.Title != "First" {
					t.Errorf("expected title 'First', got '%s'", track.Title)
				}
				if q.cursor != 0 {
					t.Errorf("expected cursor 0 after first add, got %d", q.cursor)
				}
			},
		},
		{
			name: "Next advances cursor",
			fn: func(t *testing.T) {
				q := NewQueue()
				q.Add(makeTrack("1", "First"))
				q.Add(makeTrack("2", "Second"))
				q.Add(makeTrack("3", "Third"))

				track, ok := q.Next()
				if !ok {
					t.Fatal("expected Next to return true")
				}
				if track.ID != "2" {
					t.Errorf("expected track ID '2', got '%s'", track.ID)
				}

				track, ok = q.Next()
				if !ok {
					t.Fatal("expected Next to return true")
				}
				if track.ID != "3" {
					t.Errorf("expected track ID '3', got '%s'", track.ID)
				}
			},
		},
		{
			name: "Next at end returns false",
			fn: func(t *testing.T) {
				q := NewQueue()
				q.Add(makeTrack("1", "Only"))

				_, ok := q.Next()
				if ok {
					t.Error("expected Next to return false at end")
				}
			},
		},
		{
			name: "Previous at start returns false",
			fn: func(t *testing.T) {
				q := NewQueue()
				q.Add(makeTrack("1", "First"))
				q.Add(makeTrack("2", "Second"))

				_, ok := q.Previous()
				if ok {
					t.Error("expected Previous to return false at start")
				}
			},
		},
		{
			name: "Previous moves cursor back",
			fn: func(t *testing.T) {
				q := NewQueue()
				q.Add(makeTrack("1", "First"))
				q.Add(makeTrack("2", "Second"))
				q.Add(makeTrack("3", "Third"))

				q.Next() // cursor -> 1
				q.Next() // cursor -> 2

				track, ok := q.Previous()
				if !ok {
					t.Fatal("expected Previous to return true")
				}
				if track.ID != "2" {
					t.Errorf("expected track ID '2', got '%s'", track.ID)
				}
			},
		},
		{
			name: "Remove adjusts cursor when removing before cursor",
			fn: func(t *testing.T) {
				q := NewQueue()
				q.Add(makeTrack("1", "First"))
				q.Add(makeTrack("2", "Second"))
				q.Add(makeTrack("3", "Third"))

				q.Next() // cursor -> 1 (Second)

				ok := q.Remove(0) // remove First
				if !ok {
					t.Fatal("expected Remove to return true")
				}
				if q.Len() != 2 {
					t.Errorf("expected Len 2, got %d", q.Len())
				}
				// Cursor should shift back to stay on "Second"
				track, ok := q.Current()
				if !ok {
					t.Fatal("expected Current to return true")
				}
				if track.ID != "2" {
					t.Errorf("expected current track ID '2', got '%s'", track.ID)
				}
				if q.cursor != 0 {
					t.Errorf("expected cursor 0, got %d", q.cursor)
				}
			},
		},
		{
			name: "Remove adjusts cursor when removing current track",
			fn: func(t *testing.T) {
				q := NewQueue()
				q.Add(makeTrack("1", "First"))
				q.Add(makeTrack("2", "Second"))
				q.Add(makeTrack("3", "Third"))

				q.Next() // cursor -> 1 (Second)

				ok := q.Remove(1) // remove current (Second)
				if !ok {
					t.Fatal("expected Remove to return true")
				}
				// Cursor stays at 1, now pointing to "Third"
				track, ok := q.Current()
				if !ok {
					t.Fatal("expected Current to return true")
				}
				if track.ID != "3" {
					t.Errorf("expected current track ID '3', got '%s'", track.ID)
				}
			},
		},
		{
			name: "Remove last track when cursor is on it clamps cursor",
			fn: func(t *testing.T) {
				q := NewQueue()
				q.Add(makeTrack("1", "First"))
				q.Add(makeTrack("2", "Second"))

				q.Next() // cursor -> 1 (Second)

				ok := q.Remove(1) // remove Second (last element)
				if !ok {
					t.Fatal("expected Remove to return true")
				}
				// Cursor should clamp to 0
				track, ok := q.Current()
				if !ok {
					t.Fatal("expected Current to return true")
				}
				if track.ID != "1" {
					t.Errorf("expected current track ID '1', got '%s'", track.ID)
				}
			},
		},
		{
			name: "Remove after cursor leaves cursor unchanged",
			fn: func(t *testing.T) {
				q := NewQueue()
				q.Add(makeTrack("1", "First"))
				q.Add(makeTrack("2", "Second"))
				q.Add(makeTrack("3", "Third"))

				// cursor at 0 (First)
				ok := q.Remove(2) // remove Third
				if !ok {
					t.Fatal("expected Remove to return true")
				}
				track, ok := q.Current()
				if !ok {
					t.Fatal("expected Current to return true")
				}
				if track.ID != "1" {
					t.Errorf("expected current track ID '1', got '%s'", track.ID)
				}
				if q.cursor != 0 {
					t.Errorf("expected cursor 0, got %d", q.cursor)
				}
			},
		},
		{
			name: "Remove invalid index returns false",
			fn: func(t *testing.T) {
				q := NewQueue()
				q.Add(makeTrack("1", "First"))

				if q.Remove(-1) {
					t.Error("expected Remove(-1) to return false")
				}
				if q.Remove(5) {
					t.Error("expected Remove(5) to return false")
				}
			},
		},
		{
			name: "Remove only track resets cursor to -1",
			fn: func(t *testing.T) {
				q := NewQueue()
				q.Add(makeTrack("1", "Only"))

				q.Remove(0)
				if q.cursor != -1 {
					t.Errorf("expected cursor -1, got %d", q.cursor)
				}
				if !q.IsEmpty() {
					t.Error("expected IsEmpty true after removing only track")
				}
			},
		},
		{
			name: "Clear resets everything",
			fn: func(t *testing.T) {
				q := NewQueue()
				q.Add(makeTrack("1", "First"))
				q.Add(makeTrack("2", "Second"))
				q.Next()

				q.Clear()
				if q.Len() != 0 {
					t.Errorf("expected Len 0, got %d", q.Len())
				}
				if !q.IsEmpty() {
					t.Error("expected IsEmpty true")
				}
				if q.cursor != -1 {
					t.Errorf("expected cursor -1, got %d", q.cursor)
				}
				_, ok := q.Current()
				if ok {
					t.Error("expected Current to return false after Clear")
				}
			},
		},
		{
			name: "Tracks returns a copy",
			fn: func(t *testing.T) {
				q := NewQueue()
				q.Add(makeTrack("1", "First"))
				q.Add(makeTrack("2", "Second"))

				tracks := q.Tracks()
				if len(tracks) != 2 {
					t.Fatalf("expected 2 tracks, got %d", len(tracks))
				}
				// Mutating the copy should not affect the queue.
				tracks[0].Title = "Modified"
				original, _ := q.Current()
				if original.Title == "Modified" {
					t.Error("Tracks() should return a copy, not a reference")
				}
			},
		},
		{
			name: "Next on empty queue returns false",
			fn: func(t *testing.T) {
				q := NewQueue()
				_, ok := q.Next()
				if ok {
					t.Error("expected Next to return false on empty queue")
				}
			},
		},
		{
			name: "Previous on empty queue returns false",
			fn: func(t *testing.T) {
				q := NewQueue()
				_, ok := q.Previous()
				if ok {
					t.Error("expected Previous to return false on empty queue")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.fn)
	}
}
