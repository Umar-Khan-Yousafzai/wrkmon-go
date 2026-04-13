package core

import "math/rand"

// RepeatMode controls what happens when the queue reaches the end.
type RepeatMode int

const (
	RepeatOff RepeatMode = iota // stop at end
	RepeatAll                   // loop back to start
	RepeatOne                   // replay current track
)

func (r RepeatMode) String() string {
	switch r {
	case RepeatAll:
		return "all"
	case RepeatOne:
		return "one"
	default:
		return "off"
	}
}

// Queue manages an ordered playlist of tracks.
type Queue struct {
	tracks  []Track
	cursor  int // index of currently playing track, -1 if empty
	Repeat  RepeatMode
	Shuffle bool
}

// NewQueue creates an empty queue.
func NewQueue() *Queue {
	return &Queue{cursor: -1}
}

// Add appends a track to the end of the queue.
// If this is the first track, the cursor moves to it.
func (q *Queue) Add(t Track) {
	q.tracks = append(q.tracks, t)
	if len(q.tracks) == 1 {
		q.cursor = 0
	}
}

// Remove removes the track at the given index.
// Returns false if the index is out of range.
// Adjusts the cursor to stay on the same track when possible.
func (q *Queue) Remove(index int) bool {
	if index < 0 || index >= len(q.tracks) {
		return false
	}

	q.tracks = append(q.tracks[:index], q.tracks[index+1:]...)

	if len(q.tracks) == 0 {
		q.cursor = -1
		return true
	}

	if index < q.cursor {
		// Removed before cursor: shift cursor back to stay on same track.
		q.cursor--
	} else if index == q.cursor {
		// Removed the current track: clamp cursor to valid range.
		if q.cursor >= len(q.tracks) {
			q.cursor = len(q.tracks) - 1
		}
	}
	// index > cursor: cursor unaffected.

	return true
}

// Current returns the track at the cursor.
// Returns false if the queue is empty.
func (q *Queue) Current() (Track, bool) {
	if q.cursor < 0 || q.cursor >= len(q.tracks) {
		return Track{}, false
	}
	return q.tracks[q.cursor], true
}

// Next advances the cursor and returns the next track.
// Behavior depends on repeat/shuffle mode.
func (q *Queue) Next() (Track, bool) {
	if len(q.tracks) == 0 {
		return Track{}, false
	}

	if q.Repeat == RepeatOne {
		return q.tracks[q.cursor], true
	}

	if q.Shuffle {
		q.cursor = rand.Intn(len(q.tracks))
		return q.tracks[q.cursor], true
	}

	if q.cursor+1 >= len(q.tracks) {
		if q.Repeat == RepeatAll {
			q.cursor = 0
			return q.tracks[q.cursor], true
		}
		return Track{}, false
	}
	q.cursor++
	return q.tracks[q.cursor], true
}

// Previous moves the cursor back and returns the previous track.
// Returns false if already at the beginning.
func (q *Queue) Previous() (Track, bool) {
	if q.cursor <= 0 {
		return Track{}, false
	}
	q.cursor--
	return q.tracks[q.cursor], true
}

// Tracks returns a copy of all tracks in the queue.
func (q *Queue) Tracks() []Track {
	cp := make([]Track, len(q.tracks))
	copy(cp, q.tracks)
	return cp
}

// Len returns the number of tracks in the queue.
func (q *Queue) Len() int {
	return len(q.tracks)
}

// Clear removes all tracks and resets the cursor.
func (q *Queue) Clear() {
	q.tracks = nil
	q.cursor = -1
}

// IsEmpty returns true if the queue has no tracks.
func (q *Queue) IsEmpty() bool {
	return len(q.tracks) == 0
}
