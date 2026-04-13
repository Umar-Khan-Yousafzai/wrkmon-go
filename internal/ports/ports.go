package ports

import (
	"context"

	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/core"
)

// Searcher searches YouTube for audio tracks.
type Searcher interface {
	Search(ctx context.Context, query string, limit int) ([]core.SearchResult, error)
	GetStreamURL(ctx context.Context, videoID string) (string, error)
}

// Player controls audio playback via mpv.
type Player interface {
	Play(url string) error
	Pause() error
	Resume() error
	Stop() error
	Seek(seconds float64) error
	SetVolume(vol int) error
	GetPosition() (float64, error)
	GetDuration() (float64, error)
	IsRunning() bool
	Close() error
}

// Store persists history and queue state.
type Store interface {
	SaveHistory(ctx context.Context, entry core.HistoryEntry) error
	GetHistory(ctx context.Context, limit int, offset int) ([]core.HistoryEntry, error)
	SearchHistory(ctx context.Context, query string, limit int) ([]core.HistoryEntry, error)
	SaveQueue(ctx context.Context, tracks []core.Track, cursor int) error
	LoadQueue(ctx context.Context) ([]core.Track, int, error)
	Close() error
}
