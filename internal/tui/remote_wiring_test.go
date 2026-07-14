package tui

import (
	"context"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/config"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/core"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/tui/components"
)

// remotePlayer is a minimal ports.Player stub that records Play/Pause/
// Resume/Stop calls in order, so remote-command wiring tests can assert the
// exact facade method a core.RemoteCommand routed to.
type remotePlayer struct {
	calls []string
}

func (p *remotePlayer) Play(url string) error {
	p.calls = append(p.calls, "play")
	return nil
}
func (p *remotePlayer) Pause() error {
	p.calls = append(p.calls, "pause")
	return nil
}
func (p *remotePlayer) Resume() error {
	p.calls = append(p.calls, "resume")
	return nil
}
func (p *remotePlayer) Stop() error {
	p.calls = append(p.calls, "stop")
	return nil
}
func (p *remotePlayer) Seek(seconds float64) error    { return nil }
func (p *remotePlayer) SeekTo(seconds float64) error  { return nil }
func (p *remotePlayer) SeekPercent(pct float64) error { return nil }
func (p *remotePlayer) SetVolume(vol int) error       { return nil }
func (p *remotePlayer) SetAudioFilter(f string) error { return nil }
func (p *remotePlayer) GetPosition() (float64, error) { return 0, nil }
func (p *remotePlayer) GetDuration() (float64, error) { return 0, nil }
func (p *remotePlayer) IsRunning() bool               { return true }
func (p *remotePlayer) Respawn() error                { return nil }
func (p *remotePlayer) Close() error                  { return nil }

// last returns the most recently recorded call, or "" if none yet.
func (p *remotePlayer) last() string {
	if len(p.calls) == 0 {
		return ""
	}
	return p.calls[len(p.calls)-1]
}

// fakeRemote is a core.MediaRemote test double: a buffered channel the test
// pushes commands onto (as an OS adapter would), plus a recording Publish so
// tests can assert the composed core.NowPlaying.
type fakeRemote struct {
	ch        chan core.RemoteCommand
	published []core.NowPlaying
	closed    bool
}

func newFakeRemote() *fakeRemote {
	return &fakeRemote{ch: make(chan core.RemoteCommand, 8)}
}

func (f *fakeRemote) Commands() <-chan core.RemoteCommand { return f.ch }
func (f *fakeRemote) Publish(np core.NowPlaying)          { f.published = append(f.published, np) }
func (f *fakeRemote) Close() error                        { f.closed = true; return nil }

// lastPublished returns the most recent Publish call, or the zero value if
// none happened yet.
func (f *fakeRemote) lastPublished() core.NowPlaying {
	if len(f.published) == 0 {
		return core.NowPlaying{}
	}
	return f.published[len(f.published)-1]
}

// newRemoteTestApp builds an App on a real Facade with a fake player and a
// two-track queue, already playing the first track (v1, cursor 0) — a
// common baseline for pause/next/prev/stop/publish wiring tests. remote may
// be nil to exercise the nil-safe "no remote wired" paths. player.calls is
// reset after setup so tests only see the calls their own action makes.
func newRemoteTestApp(t *testing.T, remote core.MediaRemote) (*App, *remotePlayer) {
	t.Helper()
	player := &remotePlayer{}
	f := NewFacade(mouseSearcher{}, player, mouseStore{})

	f.AddToQueue(core.SearchResult{VideoID: "v1", Title: "First Track", Channel: "Ch1", Duration: 60 * time.Second})
	f.AddToQueue(core.SearchResult{VideoID: "v2", Title: "Second Track", Channel: "Ch2", Duration: 90 * time.Second})
	if err := f.PlayFromQueue(context.Background()); err != nil {
		t.Fatalf("PlayFromQueue: %v", err)
	}
	player.calls = nil

	app := NewApp(f, config.DefaultConfig(), remote)
	return app, player
}

// drainCmds recursively executes cmd() and any cmd nested inside a returned
// tea.BatchMsg, forcing every queued side effect (e.g. the facade calls
// inside doNextTrack/doPrevTrack) to actually run. Only safe to use once
// every channel-backed cmd in the tree is guaranteed to return immediately
// (e.g. because the fakeRemote's channel has been closed) — otherwise a
// still-armed listenRemote loop would block forever.
func drainCmds(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		return
	}
	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		for _, c := range batch {
			drainCmds(t, c)
		}
	}
}

// TestListenRemoteDeliversQueuedCommand proves the buffered-channel
// injection itself works: a command pushed onto the fake remote's channel
// comes back out of App.listenRemote() wrapped as a remoteCmdMsg.
func TestListenRemoteDeliversQueuedCommand(t *testing.T) {
	remote := newFakeRemote()
	app, _ := newRemoteTestApp(t, remote)

	remote.ch <- core.RemoteNext

	cmd := app.listenRemote()
	if cmd == nil {
		t.Fatal("listenRemote() returned nil cmd though a remote is wired")
	}
	msg := cmd() // buffered channel already has a queued item: returns immediately
	rc, ok := msg.(remoteCmdMsg)
	if !ok {
		t.Fatalf("listenRemote() cmd produced %T, want remoteCmdMsg", msg)
	}
	if rc.cmd != core.RemoteNext {
		t.Errorf("remoteCmdMsg.cmd = %v, want RemoteNext", rc.cmd)
	}
}

func TestRemotePlayPauseTogglesPauseWhenPlaying(t *testing.T) {
	remote := newFakeRemote()
	app, player := newRemoteTestApp(t, remote)

	_, cmd := app.Update(remoteCmdMsg{cmd: core.RemotePlayPause})

	if player.last() != "pause" {
		t.Errorf("player recorded %q, want pause (track was playing)", player.last())
	}
	if app.facade.State().Status != core.StatusPaused {
		t.Errorf("facade status = %v, want paused", app.facade.State().Status)
	}
	if cmd == nil {
		t.Fatal("expected Update to return a non-nil cmd (the re-armed listen loop)")
	}
}

func TestRemotePlayPauseResumesWhenPaused(t *testing.T) {
	remote := newFakeRemote()
	app, player := newRemoteTestApp(t, remote)
	if err := app.facade.Pause(); err != nil {
		t.Fatalf("setup Pause: %v", err)
	}
	player.calls = nil // isolate the toggle under test

	app.Update(remoteCmdMsg{cmd: core.RemotePlayPause})

	if player.last() != "resume" {
		t.Errorf("player recorded %q, want resume (track was paused)", player.last())
	}
	if app.facade.State().Status != core.StatusPlaying {
		t.Errorf("facade status = %v, want playing", app.facade.State().Status)
	}
}

func TestRemoteStopCallsFacadeStop(t *testing.T) {
	remote := newFakeRemote()
	app, player := newRemoteTestApp(t, remote)

	app.Update(remoteCmdMsg{cmd: core.RemoteStop})

	if player.last() != "stop" {
		t.Errorf("player recorded %q, want stop", player.last())
	}
	if app.facade.State().Status != core.StatusStopped {
		t.Errorf("facade status = %v, want stopped", app.facade.State().Status)
	}
	if app.facade.State().Current != nil {
		t.Error("expected facade.State().Current to be nil after RemoteStop")
	}
}

func TestRemoteNextRoutesToNextTrack(t *testing.T) {
	remote := newFakeRemote()
	app, player := newRemoteTestApp(t, remote)
	close(remote.ch) // any re-armed listen resolves immediately instead of blocking

	_, cmd := app.Update(remoteCmdMsg{cmd: core.RemoteNext})
	drainCmds(t, cmd)

	if got := app.facade.State().Current; got == nil || got.VideoID != "v2" {
		t.Fatalf("facade.State().Current = %+v, want the v2 track after RemoteNext", got)
	}
	if player.last() != "play" {
		t.Errorf("player recorded %q, want play", player.last())
	}
}

func TestRemotePrevRoutesToPreviousTrack(t *testing.T) {
	remote := newFakeRemote()
	app, player := newRemoteTestApp(t, remote)
	if err := app.facade.NextTrack(context.Background()); err != nil {
		t.Fatalf("setup NextTrack to v2: %v", err)
	}
	player.calls = nil
	close(remote.ch)

	_, cmd := app.Update(remoteCmdMsg{cmd: core.RemotePrev})
	drainCmds(t, cmd)

	if got := app.facade.State().Current; got == nil || got.VideoID != "v1" {
		t.Fatalf("facade.State().Current = %+v, want the v1 track after RemotePrev", got)
	}
	if player.last() != "play" {
		t.Errorf("player recorded %q, want play", player.last())
	}
}

func TestPublishNowPlayingComposesFromFacadeState(t *testing.T) {
	remote := newFakeRemote()
	app, _ := newRemoteTestApp(t, remote)
	app.currentPos = 12.5
	app.currentDur = 60

	app.publishNowPlaying()

	np := remote.lastPublished()
	if np.Title != "First Track" {
		t.Errorf("NowPlaying.Title = %q, want %q", np.Title, "First Track")
	}
	if np.Artist != "Ch1" {
		t.Errorf("NowPlaying.Artist = %q, want %q", np.Artist, "Ch1")
	}
	if !np.Playing {
		t.Error("NowPlaying.Playing = false, want true (facade state is playing)")
	}
	if np.Position != 12500*time.Millisecond {
		t.Errorf("NowPlaying.Position = %v, want 12.5s", np.Position)
	}
	if np.Duration != 60*time.Second {
		t.Errorf("NowPlaying.Duration = %v, want 60s", np.Duration)
	}
}

// TestSlashStopResetsPositionAndPublishesZeroed is the regression test for
// the inconsistent-stop bug: /stop at 45s of a 180s track used to publish
// NowPlaying{Playing:false, Position:45s, Duration:180s} — stopped but with
// a stale timestamp (which also fed the status bar) — while RemoteStop and
// the auto-advance-exhausted path correctly reported 0/0. All three sites
// now share doStopPlayback, so /stop must publish a fully zeroed snapshot.
func TestSlashStopResetsPositionAndPublishesZeroed(t *testing.T) {
	remote := newFakeRemote()
	app, player := newRemoteTestApp(t, remote)
	app.currentPos = 45
	app.currentDur = 180

	app.Update(components.PromptSubmitMsg{Value: "/stop"})

	if player.last() != "stop" {
		t.Fatalf("player recorded %q, want stop", player.last())
	}
	np := remote.lastPublished()
	if np.Playing {
		t.Error("published NowPlaying.Playing = true, want false after /stop")
	}
	if np.Position != 0 {
		t.Errorf("published NowPlaying.Position = %v, want 0 (stale 45s must be reset)", np.Position)
	}
	if np.Duration != 0 {
		t.Errorf("published NowPlaying.Duration = %v, want 0 (stale 180s must be reset)", np.Duration)
	}
	if np.Title != "" {
		t.Errorf("published NowPlaying.Title = %q, want empty after stop", np.Title)
	}
	if app.currentPos != 0 || app.currentDur != 0 {
		t.Errorf("app.currentPos/currentDur = %v/%v, want 0/0 (stale values also feed the status bar)",
			app.currentPos, app.currentDur)
	}
}

// TestRemoteStopPublishesZeroedSnapshot pins the already-correct RemoteStop
// behavior so the shared-helper refactor can't regress it.
func TestRemoteStopPublishesZeroedSnapshot(t *testing.T) {
	remote := newFakeRemote()
	app, _ := newRemoteTestApp(t, remote)
	app.currentPos = 45
	app.currentDur = 180

	app.Update(remoteCmdMsg{cmd: core.RemoteStop})

	np := remote.lastPublished()
	if np.Playing || np.Position != 0 || np.Duration != 0 {
		t.Errorf("published NowPlaying = %+v, want zeroed stopped snapshot", np)
	}
}

// TestPublishNowPlayingNilSafeWithoutRemote guards the "no remote wired"
// path (e.g. media_keys=false, see the remoteListenCmd tests below): calling
// publishNowPlaying must never panic just because a.remote is nil.
func TestPublishNowPlayingNilSafeWithoutRemote(t *testing.T) {
	app, _ := newRemoteTestApp(t, nil)
	app.publishNowPlaying()
}

func TestRemoteListenCmdNilWhenMediaKeysDisabled(t *testing.T) {
	remote := newFakeRemote()
	app, _ := newRemoteTestApp(t, remote)
	app.cfg.MediaKeys = false

	if cmd := app.remoteListenCmd(); cmd != nil {
		t.Error("expected nil cmd when config.MediaKeys is false, even with a remote wired")
	}
}

func TestRemoteListenCmdStartsWhenEnabledAndWired(t *testing.T) {
	remote := newFakeRemote()
	app, _ := newRemoteTestApp(t, remote)
	app.cfg.MediaKeys = true

	if cmd := app.remoteListenCmd(); cmd == nil {
		t.Error("expected a non-nil cmd when MediaKeys is enabled and a remote is wired")
	}
}

func TestRemoteListenCmdNilWhenNoRemoteWired(t *testing.T) {
	app, _ := newRemoteTestApp(t, nil)
	app.cfg.MediaKeys = true

	if cmd := app.remoteListenCmd(); cmd != nil {
		t.Error("expected nil cmd when no remote was ever wired to NewApp")
	}
}
