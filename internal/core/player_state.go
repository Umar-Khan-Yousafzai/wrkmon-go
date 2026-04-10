package core

import "time"

// PlaybackStatus represents the current state of the player.
type PlaybackStatus int

const (
	StatusStopped PlaybackStatus = iota
	StatusPlaying
	StatusPaused
)

func (s PlaybackStatus) String() string {
	switch s {
	case StatusPlaying:
		return "playing"
	case StatusPaused:
		return "paused"
	default:
		return "stopped"
	}
}

// PlayerState holds the current playback state.
type PlayerState struct {
	Status   PlaybackStatus
	Current  *Track
	Position time.Duration
	Duration time.Duration
	Volume   int // 0-100
}
