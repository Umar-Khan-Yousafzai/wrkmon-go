package tui

import (
	"context"
	"strings"
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
func (p *mousePlayer) SetAudioFilter(f string) error { return nil }
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

// newNowPlayingApp spins up an App on the now-playing view with a playing
// track and one rendered frame, so the big bar's geometry has been captured.
func newNowPlayingApp(t *testing.T) (*App, *mousePlayer) {
	t.Helper()

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
	app.currentView = viewNowPlaying
	app.Update(PositionUpdateMsg{Position: 10, Duration: 100})

	// Render the now-playing view so the bar geometry is captured.
	_ = app.View()

	if app.npBarWidth <= 0 {
		t.Fatalf("expected npBarWidth > 0 after render, got %d", app.npBarWidth)
	}
	return app, player
}

// TestMouseNowPlayingBarClickAndOverlayGating clicks the big now-playing
// progress bar and asserts the seek lands near the expected percentage,
// then opens the help overlay and asserts the same click no longer seeks
// (the overlay covers the bar, so its stale geometry must not be clickable).
func TestMouseNowPlayingBarClickAndOverlayGating(t *testing.T) {
	app, player := newNowPlayingApp(t)

	clickX := app.npBarStart + app.npBarWidth/2
	app.Update(tea.MouseMsg{
		X:      clickX,
		Y:      app.npBarRow,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})

	if player.lastPct < 48 || player.lastPct > 52 {
		t.Errorf("SeekPercent called with %.2f, want ~50", player.lastPct)
	}

	// Release to end the drag started by the press above.
	app.Update(tea.MouseMsg{
		X:      clickX,
		Y:      app.npBarRow,
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
	})

	// With the help overlay open, the same click must NOT seek.
	player.lastPct = -1
	app.helpText = "help"
	app.Update(tea.MouseMsg{
		X:      clickX,
		Y:      app.npBarRow,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})
	if player.lastPct != -1 {
		t.Errorf("expected no seek while help overlay open, got %.2f", player.lastPct)
	}
}

// TestNowPlayingBarGeometryStableWithLongTitle guards against a regression
// where a title (or channel) too long to fit the terminal width wrapped onto
// extra visual rows when rendered through renderContent, but npBarRow —
// computed from a raw "\n" count on the unwrapped builder string — couldn't
// see those wrapped rows. The bar's real on-screen row then drifted below
// npBarRow, so a real click at the bar's true visual position (which is all
// a real terminal ever reports) landed on a Y the click handler didn't
// recognize as the bar, and the click silently did nothing.
//
// The test renders through App.View() (the same renderContent path that
// wraps at a.width) with a 200-char title/channel at width 80, independently
// locates the bar's true visual row by scanning the rendered text for the
// progress-bar's line-drawing characters (NOT by trusting app.npBarRow),
// and clicks there. Before the fix this row wouldn't match app.npBarRow and
// the seek would never fire; after the fix (title/channel truncated to one
// row each) the two agree and the seek lands ~50%.
func TestNowPlayingBarGeometryStableWithLongTitle(t *testing.T) {
	player := &mousePlayer{}
	f := NewFacade(mouseSearcher{}, player, mouseStore{})

	if err := f.PlayTrack(context.Background(), core.Track{
		VideoID:  "v",
		Title:    strings.Repeat("x", 200),
		Channel:  strings.Repeat("y", 200),
		Duration: 100 * time.Second,
	}); err != nil {
		t.Fatalf("PlayTrack: %v", err)
	}

	app := NewApp(f, config.DefaultConfig())
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 30})
	app.currentView = viewNowPlaying
	app.Update(PositionUpdateMsg{Position: 10, Duration: 100})

	// Render through App.View(), not renderNowPlayingView() directly, so
	// the terminal-width wrapping that triggers the regression actually runs.
	rendered := app.View()

	if app.npBarWidth <= 0 {
		t.Fatalf("expected npBarWidth > 0 after render, got %d", app.npBarWidth)
	}

	// Locate the bar's true visual row independently of app.npBarRow: it's
	// the only line containing the progress-bar's heavy/light horizontal
	// line-drawing runes (see progressBar()).
	visualRow := -1
	for i, line := range strings.Split(rendered, "\n") {
		if strings.ContainsRune(line, '━') || strings.ContainsRune(line, '─') {
			visualRow = i
			break
		}
	}
	if visualRow == -1 {
		t.Fatal("could not locate the progress bar's rendered row")
	}

	clickX := app.npBarStart + app.npBarWidth/2
	app.Update(tea.MouseMsg{
		X:      clickX,
		Y:      visualRow,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})

	if player.lastPct < 48 || player.lastPct > 52 {
		t.Errorf("SeekPercent called with %.2f, want ~50 (click at the bar's true visual row %d didn't register; app.npBarRow=%d)",
			player.lastPct, visualRow, app.npBarRow)
	}
}

// TestMouseNowPlayingBarDragUsesCapturedGeometry presses on the big bar and
// then drags to its last column, asserting the motion seek maps against the
// geometry captured at press time (dragStart/dragWidth), not the statusbar's.
func TestMouseNowPlayingBarDragUsesCapturedGeometry(t *testing.T) {
	app, player := newNowPlayingApp(t)

	app.Update(tea.MouseMsg{
		X:      app.npBarStart,
		Y:      app.npBarRow,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})

	if !app.seekDragging {
		t.Fatal("press on now-playing bar did not start a drag")
	}
	if app.dragStart != app.npBarStart || app.dragWidth != app.npBarWidth {
		t.Errorf("press captured dragStart/dragWidth = %d/%d, want %d/%d",
			app.dragStart, app.dragWidth, app.npBarStart, app.npBarWidth)
	}

	player.lastPct = -1
	app.lastDragSeek = time.Time{} // bypass the 250ms drag throttle
	app.Update(tea.MouseMsg{
		X:      app.npBarStart + app.npBarWidth - 1,
		Y:      app.npBarRow,
		Action: tea.MouseActionMotion,
	})
	if player.lastPct < 98 {
		t.Errorf("expected drag to seek near 100%%, got %.2f", player.lastPct)
	}

	app.Update(tea.MouseMsg{
		X:      app.npBarStart + app.npBarWidth - 1,
		Y:      app.npBarRow,
		Action: tea.MouseActionRelease,
		Button: tea.MouseButtonLeft,
	})
	if app.seekDragging {
		t.Error("seekDragging still true after release")
	}
}
