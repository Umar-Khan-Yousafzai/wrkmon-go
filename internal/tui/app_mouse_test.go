package tui

import (
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/config"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/core"
)

// mousePlayer is a minimal ports.Player stub used only to exercise the
// mouse-driven seekbar path. It records the last percentage passed to
// SeekPercent so the test can assert the click landed near the midpoint.
type mousePlayer struct {
	lastPct float64
}

func (p *mousePlayer) Play(url string) error        { return nil }
func (p *mousePlayer) Pause() error                 { return nil }
func (p *mousePlayer) Resume() error                { return nil }
func (p *mousePlayer) Stop() error                  { return nil }
func (p *mousePlayer) Seek(seconds float64) error   { return nil }
func (p *mousePlayer) SeekTo(seconds float64) error { return nil }
func (p *mousePlayer) SeekPercent(pct float64) error {
	p.lastPct = pct
	return nil
}
func (p *mousePlayer) SetVolume(vol int) error       { return nil }
func (p *mousePlayer) GetPosition() (float64, error) { return 0, nil }
func (p *mousePlayer) GetDuration() (float64, error) { return 0, nil }
func (p *mousePlayer) IsRunning() bool               { return true }
func (p *mousePlayer) Respawn() error                { return nil }
func (p *mousePlayer) Close() error                  { return nil }

// mouseSearcher is a no-op ports.Searcher stub.
type mouseSearcher struct{}

func (mouseSearcher) Search(ctx context.Context, query string, limit int) ([]core.SearchResult, error) {
	return nil, nil
}

func (mouseSearcher) GetStreamURL(ctx context.Context, videoID string) (string, error) {
	return "http://example.invalid/" + videoID, nil
}

// mouseStore is a no-op ports.Store stub.
type mouseStore struct{}

func (mouseStore) SaveHistory(ctx context.Context, entry core.HistoryEntry) error { return nil }

func (mouseStore) GetHistory(ctx context.Context, limit int, offset int) ([]core.HistoryEntry, error) {
	return nil, nil
}

func (mouseStore) SearchHistory(ctx context.Context, query string, limit int) ([]core.HistoryEntry, error) {
	return nil, nil
}

func (mouseStore) SaveQueue(ctx context.Context, tracks []core.Track, cursor int) error { return nil }

func (mouseStore) LoadQueue(ctx context.Context) ([]core.Track, int, error) { return nil, 0, nil }

func (mouseStore) CacheSearchResults(ctx context.Context, query string, results []core.SearchResult, ttl time.Duration) error {
	return nil
}

func (mouseStore) GetCachedSearch(ctx context.Context, query string) ([]core.SearchResult, bool, error) {
	return nil, false, nil
}

func (mouseStore) CacheLyrics(ctx context.Context, videoID, lyrics string) error { return nil }

func (mouseStore) GetCachedLyrics(ctx context.Context, videoID string) (string, bool, error) {
	return "", false, nil
}

func (mouseStore) SaveDownload(ctx context.Context, dl core.Download) error { return nil }

func (mouseStore) ListDownloads(ctx context.Context, limit int) ([]core.Download, error) {
	return nil, nil
}

func (mouseStore) CreatePlaylist(ctx context.Context, name string) (core.Playlist, error) {
	return core.Playlist{}, nil
}

func (mouseStore) ListPlaylists(ctx context.Context) ([]core.Playlist, error) { return nil, nil }

func (mouseStore) GetPlaylist(ctx context.Context, id int) (core.Playlist, error) {
	return core.Playlist{}, nil
}

func (mouseStore) DeletePlaylist(ctx context.Context, id int) error { return nil }

func (mouseStore) AddToPlaylist(ctx context.Context, playlistID int, track core.Track) error {
	return nil
}

func (mouseStore) RemoveFromPlaylist(ctx context.Context, playlistID int, position int) error {
	return nil
}

func (mouseStore) Close() error { return nil }

// TestMouseSeekbarClickSyncsStatusBar drives a real mouse click on the
// statusbar mini seekbar through App.Update and asserts (1) the player
// receives the expected SeekPercent, (2) the statusbar's rendered View
// changes immediately (proving the live redraw sync point runs on the
// mouse path, not just on the next 1s position tick), and (3) currentPos
// is updated to match.
func TestMouseSeekbarClickSyncsStatusBar(t *testing.T) {
	player := &mousePlayer{}
	f := NewFacade(mouseSearcher{}, player, mouseStore{})

	if err := f.PlayTrack(context.Background(), core.Track{
		VideoID:  "v",
		Title:    "x",
		Duration: 100 * time.Second,
	}); err != nil {
		t.Fatalf("PlayTrack: %v", err)
	}

	app := NewApp(f, config.DefaultConfig())

	app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	app.Update(PositionUpdateMsg{Position: 10, Duration: 100})

	start, width, ok := app.statusBar.BarBounds()
	if !ok {
		t.Fatal("expected a seekbar to be drawn at width 100 while playing")
	}

	preClick := app.statusBar.View()

	statusRow := app.height - lipgloss.Height(app.prompt.View()) - 1
	clickX := start + width/2

	app.Update(tea.MouseMsg{
		X:      clickX,
		Y:      statusRow,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})

	if player.lastPct < 48 || player.lastPct > 52 {
		t.Errorf("SeekPercent called with %.2f, want ~50", player.lastPct)
	}

	postClick := app.statusBar.View()
	if postClick == preClick {
		t.Error("statusbar View() did not change after mouse seek; live redraw did not happen")
	}

	const wantPos = 50.0
	if app.currentPos < wantPos-2 || app.currentPos > wantPos+2 {
		t.Errorf("app.currentPos = %.2f, want ~%.2f", app.currentPos, wantPos)
	}
}
