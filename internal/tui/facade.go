package tui

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/adapters/lyrics"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/core"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/ports"
)

// Facade mediates between the TUI and core domain + adapters.
type Facade struct {
	searcher ports.Searcher
	player   ports.Player
	store    ports.Store
	lyrics   *lyrics.Fetcher
	queue    *core.Queue
	state    core.PlayerState
}

// NewFacade creates the facade with injected adapters.
func NewFacade(searcher ports.Searcher, player ports.Player, store ports.Store) *Facade {
	return &Facade{
		searcher: searcher,
		player:   player,
		store:    store,
		lyrics:   lyrics.NewFetcher(),
		queue:    core.NewQueue(),
		state: core.PlayerState{
			Volume: 50,
		},
	}
}

// Search performs a YouTube search, using cache when available.
func (f *Facade) Search(ctx context.Context, query string, limit int) ([]core.SearchResult, error) {
	// Check cache first
	if cached, ok, _ := f.store.GetCachedSearch(ctx, query); ok && len(cached) > 0 {
		if len(cached) > limit {
			cached = cached[:limit]
		}
		return cached, nil
	}

	results, err := f.searcher.Search(ctx, query, limit)
	if err != nil {
		return nil, err
	}

	// Cache results for 1 hour (best-effort)
	_ = f.store.CacheSearchResults(ctx, query, results, time.Hour)

	return results, nil
}

// PlayTrack resolves the stream URL and starts playback.
func (f *Facade) PlayTrack(ctx context.Context, track core.Track) error {
	url, err := f.searcher.GetStreamURL(ctx, track.VideoID)
	if err != nil {
		return fmt.Errorf("get stream URL: %w", err)
	}

	if err := f.player.Play(url); err != nil {
		return fmt.Errorf("play: %w", err)
	}

	track.StreamURL = url
	f.state.Status = core.StatusPlaying
	f.state.Current = &track

	// Record in history
	entry := core.HistoryEntry{
		ID:       uuid.New().String(),
		Track:    track,
		PlayedAt: time.Now(),
	}
	// Best-effort history save
	_ = f.store.SaveHistory(ctx, entry)

	return nil
}

// AddToQueue adds a search result to the queue.
func (f *Facade) AddToQueue(result core.SearchResult) core.Track {
	track := result.ToTrack(uuid.New().String())
	f.queue.Add(track)
	return track
}

// PlayFromQueue plays the track at the queue cursor.
func (f *Facade) PlayFromQueue(ctx context.Context) error {
	track, ok := f.queue.Current()
	if !ok {
		return fmt.Errorf("queue is empty")
	}
	return f.PlayTrack(ctx, track)
}

// NextTrack advances queue and plays next.
func (f *Facade) NextTrack(ctx context.Context) error {
	_, ok := f.queue.Next()
	if !ok {
		return fmt.Errorf("no next track")
	}
	return f.PlayFromQueue(ctx)
}

// PreviousTrack goes back in queue and plays.
func (f *Facade) PreviousTrack(ctx context.Context) error {
	_, ok := f.queue.Previous()
	if !ok {
		return fmt.Errorf("no previous track")
	}
	return f.PlayFromQueue(ctx)
}

// Pause pauses playback.
func (f *Facade) Pause() error {
	if err := f.player.Pause(); err != nil {
		return err
	}
	f.state.Status = core.StatusPaused
	return nil
}

// Resume resumes playback.
func (f *Facade) Resume() error {
	if err := f.player.Resume(); err != nil {
		return err
	}
	f.state.Status = core.StatusPlaying
	return nil
}

// TogglePause toggles between play and pause.
func (f *Facade) TogglePause() error {
	if f.state.Status == core.StatusPlaying {
		return f.Pause()
	}
	return f.Resume()
}

// Stop stops playback.
func (f *Facade) Stop() error {
	if err := f.player.Stop(); err != nil {
		return err
	}
	f.state.Status = core.StatusStopped
	f.state.Current = nil
	return nil
}

// Seek seeks by the given number of seconds (positive = forward).
func (f *Facade) Seek(seconds float64) error {
	return f.player.Seek(seconds)
}

// SetVolume sets the volume (0-100).
func (f *Facade) SetVolume(vol int) error {
	if vol < 0 {
		vol = 0
	}
	if vol > 100 {
		vol = 100
	}
	if err := f.player.SetVolume(vol); err != nil {
		return err
	}
	f.state.Volume = vol
	return nil
}

// VolumeUp increases volume by 5.
func (f *Facade) VolumeUp() error {
	return f.SetVolume(f.state.Volume + 5)
}

// VolumeDown decreases volume by 5.
func (f *Facade) VolumeDown() error {
	return f.SetVolume(f.state.Volume - 5)
}

// GetPosition returns current playback position in seconds.
func (f *Facade) GetPosition() (float64, error) {
	return f.player.GetPosition()
}

// GetDuration returns total duration of current track in seconds.
func (f *Facade) GetDuration() (float64, error) {
	return f.player.GetDuration()
}

// IsPlaying reports whether the player process is still running.
func (f *Facade) IsPlaying() bool {
	return f.player.IsRunning()
}

// Respawn attempts to restart the player with the last played URL.
func (f *Facade) Respawn() error {
	if err := f.player.Respawn(); err != nil {
		return err
	}
	f.state.Status = core.StatusPlaying
	return nil
}

// State returns current player state.
func (f *Facade) State() core.PlayerState { return f.state }

// Queue returns the queue.
func (f *Facade) Queue() *core.Queue { return f.queue }

// LoadHistory loads history from the store.
func (f *Facade) LoadHistory(ctx context.Context, limit, offset int) ([]core.HistoryEntry, error) {
	return f.store.GetHistory(ctx, limit, offset)
}

// SearchHistory searches history by query.
func (f *Facade) SearchHistory(ctx context.Context, query string, limit int) ([]core.HistoryEntry, error) {
	return f.store.SearchHistory(ctx, query, limit)
}

// SaveQueueState persists the current queue to the store.
func (f *Facade) SaveQueueState(ctx context.Context) error {
	return f.store.SaveQueue(ctx, f.queue.Tracks(), 0)
}

// LoadQueueState loads the queue from the store.
func (f *Facade) LoadQueueState(ctx context.Context) error {
	tracks, cursor, err := f.store.LoadQueue(ctx)
	if err != nil {
		return err
	}
	f.queue.Clear()
	for _, t := range tracks {
		f.queue.Add(t)
	}
	// Set cursor position by advancing
	for i := 0; i < cursor; i++ {
		f.queue.Next()
	}
	return nil
}

// CreatePlaylist creates a new playlist.
func (f *Facade) CreatePlaylist(ctx context.Context, name string) (core.Playlist, error) {
	return f.store.CreatePlaylist(ctx, name)
}

// ListPlaylists returns all playlists.
func (f *Facade) ListPlaylists(ctx context.Context) ([]core.Playlist, error) {
	return f.store.ListPlaylists(ctx)
}

// GetPlaylist returns a playlist with tracks.
func (f *Facade) GetPlaylist(ctx context.Context, id int) (core.Playlist, error) {
	return f.store.GetPlaylist(ctx, id)
}

// DeletePlaylist removes a playlist.
func (f *Facade) DeletePlaylist(ctx context.Context, id int) error {
	return f.store.DeletePlaylist(ctx, id)
}

// AddToPlaylist adds a track to a playlist.
func (f *Facade) AddToPlaylist(ctx context.Context, playlistID int, track core.Track) error {
	return f.store.AddToPlaylist(ctx, playlistID, track)
}

// PlayPlaylist loads all tracks from a playlist into the queue and starts playing.
func (f *Facade) PlayPlaylist(ctx context.Context, id int) error {
	p, err := f.store.GetPlaylist(ctx, id)
	if err != nil {
		return err
	}
	if len(p.Tracks) == 0 {
		return fmt.Errorf("playlist %q is empty", p.Name)
	}
	f.queue.Clear()
	for _, t := range p.Tracks {
		t.ID = uuid.New().String()
		f.queue.Add(t)
	}
	return f.PlayFromQueue(ctx)
}

// FetchLyrics fetches lyrics for a track, using cache when available.
func (f *Facade) FetchLyrics(ctx context.Context, videoID, title, channel string) (string, error) {
	// Check cache
	if cached, ok, _ := f.store.GetCachedLyrics(ctx, videoID); ok {
		return cached, nil
	}
	// Fetch from API
	text, err := f.lyrics.Fetch(ctx, title, channel)
	if err != nil {
		return "", err
	}
	// Cache result
	_ = f.store.CacheLyrics(ctx, videoID, text)
	return text, nil
}

// DownloadTrack downloads audio for a track to the given directory.
func (f *Facade) DownloadTrack(ctx context.Context, videoID, title, channel string, outputDir string) (string, error) {
	type downloader interface {
		Download(ctx context.Context, videoID string, outputDir string) (string, error)
	}
	d, ok := f.searcher.(downloader)
	if !ok {
		return "", fmt.Errorf("searcher does not support downloads")
	}
	filePath, err := d.Download(ctx, videoID, outputDir)
	if err != nil {
		return "", err
	}
	// Record in DB
	dl := core.Download{
		VideoID:      videoID,
		Title:        title,
		Channel:      channel,
		FilePath:     filePath,
		DownloadedAt: time.Now(),
	}
	_ = f.store.SaveDownload(ctx, dl)
	return filePath, nil
}

// ListDownloads returns recent downloads.
func (f *Facade) ListDownloads(ctx context.Context, limit int) ([]core.Download, error) {
	return f.store.ListDownloads(ctx, limit)
}

// UpdateYtDlp runs yt-dlp self-update if using bundled binary.
func (f *Facade) UpdateYtDlp(ctx context.Context) (string, error) {
	type updater interface {
		Update(ctx context.Context) (string, error)
	}
	if u, ok := f.searcher.(updater); ok {
		return u.Update(ctx)
	}
	return "yt-dlp update not supported with this searcher", nil
}

// Close shuts down all adapters.
func (f *Facade) Close() error {
	f.player.Close()
	return f.store.Close()
}
