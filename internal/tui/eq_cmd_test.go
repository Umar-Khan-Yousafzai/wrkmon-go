package tui

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/config"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/core"
)

// eqPlayer is a minimal ports.Player stub that records every filter string
// passed to SetAudioFilter, in call order. Used to verify /eq commands and
// reapply-on-track-start actually reach the player.
type eqPlayer struct {
	filters []string
}

func (p *eqPlayer) Play(url string) error         { return nil }
func (p *eqPlayer) Pause() error                  { return nil }
func (p *eqPlayer) Resume() error                 { return nil }
func (p *eqPlayer) Stop() error                   { return nil }
func (p *eqPlayer) Seek(seconds float64) error    { return nil }
func (p *eqPlayer) SeekTo(seconds float64) error  { return nil }
func (p *eqPlayer) SeekPercent(pct float64) error { return nil }
func (p *eqPlayer) SetVolume(vol int) error       { return nil }
func (p *eqPlayer) SetAudioFilter(f string) error {
	p.filters = append(p.filters, f)
	return nil
}
func (p *eqPlayer) GetPosition() (float64, error) { return 0, nil }
func (p *eqPlayer) GetDuration() (float64, error) { return 0, nil }
func (p *eqPlayer) IsRunning() bool               { return true }
func (p *eqPlayer) Respawn() error                { return nil }
func (p *eqPlayer) Close() error                  { return nil }

// last returns the most recently recorded filter, or "" if none yet.
func (p *eqPlayer) last() string {
	if len(p.filters) == 0 {
		return ""
	}
	return p.filters[len(p.filters)-1]
}

// erroringEQPlayer is an eqPlayer whose SetAudioFilter fails, with a
// configurable IsRunning, so /eq tests can distinguish "apply failed while
// nothing is playing" (tolerated) from "apply failed while playing"
// (surfaced). It embeds eqPlayer so it still records the filter it was asked
// to apply.
type erroringEQPlayer struct {
	eqPlayer
	running   bool
	filterErr error
}

func (p *erroringEQPlayer) IsRunning() bool { return p.running }
func (p *erroringEQPlayer) SetAudioFilter(f string) error {
	p.filters = append(p.filters, f)
	return p.filterErr
}

// newEQTestApp builds an App wired to a fake player via a real Facade, with
// config persistence redirected to a temp HOME so /eq's saveEQ() doesn't
// touch the developer's real config file.
func newEQTestApp(t *testing.T) (*App, *eqPlayer) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp) // windows

	player := &eqPlayer{}
	f := NewFacade(mouseSearcher{}, player, mouseStore{})
	app := NewApp(f, config.DefaultConfig())
	return app, player
}

func TestEQBassPresetSetsFilterAndFeedback(t *testing.T) {
	app, player := newEQTestApp(t)

	out, handled, err := app.dispatch.Execute("/eq bass")
	if err != nil {
		t.Fatalf("execute /eq bass: %v", err)
	}
	if !handled {
		t.Fatal("expected /eq to be handled")
	}
	if !strings.Contains(player.last(), "superequalizer") {
		t.Errorf("recorded filter = %q, want it to contain superequalizer", player.last())
	}
	if !strings.Contains(strings.ToLower(out), "bass") {
		t.Errorf("feedback = %q, want it to mention bass", out)
	}
}

func TestEQOffClearsFilter(t *testing.T) {
	app, player := newEQTestApp(t)

	if _, _, err := app.dispatch.Execute("/eq bass"); err != nil {
		t.Fatalf("setup /eq bass: %v", err)
	}

	out, _, err := app.dispatch.Execute("/eq off")
	if err != nil {
		t.Fatalf("execute /eq off: %v", err)
	}
	if player.last() != "" {
		t.Errorf("recorded filter after /eq off = %q, want empty", player.last())
	}
	if !strings.Contains(strings.ToLower(out), "off") {
		t.Errorf("feedback = %q, want it to mention off", out)
	}
}

func TestEQBandSetsCustomBandAndFeedback(t *testing.T) {
	app, player := newEQTestApp(t)

	out, _, err := app.dispatch.Execute("/eq band 3 4.5")
	if err != nil {
		t.Fatalf("execute /eq band 3 4.5: %v", err)
	}
	if player.last() == "" {
		t.Error("expected a filter to be recorded for a custom band")
	}
	if !strings.Contains(out, "3") {
		t.Errorf("feedback = %q, want it to mention band 3", out)
	}
}

func TestEQBandOutOfRangeErrors(t *testing.T) {
	app, _ := newEQTestApp(t)

	_, _, err := app.dispatch.Execute("/eq band 25 1")
	if err == nil {
		t.Fatal("expected an error for band 25 (out of 1-18 range)")
	}
}

func TestEQUnknownPresetListsValidNames(t *testing.T) {
	app, _ := newEQTestApp(t)

	_, _, err := app.dispatch.Execute("/eq nope")
	if err == nil {
		t.Fatal("expected an error for unknown preset")
	}
	for _, name := range core.EQPresetNames {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q should list valid preset %q", err.Error(), name)
		}
	}
}

func TestEQShowAndBareContainPresetName(t *testing.T) {
	app, _ := newEQTestApp(t)

	if _, _, err := app.dispatch.Execute("/eq bass"); err != nil {
		t.Fatalf("setup /eq bass: %v", err)
	}

	out, _, err := app.dispatch.Execute("/eq show")
	if err != nil {
		t.Fatalf("execute /eq show: %v", err)
	}
	if !strings.Contains(out, "bass") {
		t.Errorf("/eq show output = %q, want it to contain preset name bass", out)
	}

	outBare, _, err := app.dispatch.Execute("/eq")
	if err != nil {
		t.Fatalf("execute bare /eq: %v", err)
	}
	if !strings.Contains(outBare, "bass") {
		t.Errorf("bare /eq output = %q, want it to contain preset name bass", outBare)
	}
}

func TestEQFilterReappliedOnTrackStart(t *testing.T) {
	app, player := newEQTestApp(t)

	if _, _, err := app.dispatch.Execute("/eq bass"); err != nil {
		t.Fatalf("setup /eq bass: %v", err)
	}
	wantFilter := app.eq.FilterString()
	if wantFilter == "" {
		t.Fatal("test setup: expected a non-empty filter for bass preset")
	}

	// Isolate the reapply-on-play behaviour from the /eq bass call above.
	player.filters = nil

	if err := app.facade.PlayTrack(context.Background(), core.Track{
		VideoID:  "v",
		Title:    "x",
		Duration: 100 * time.Second,
	}); err != nil {
		t.Fatalf("PlayTrack: %v", err)
	}

	if player.last() != wantFilter {
		t.Errorf("filter recorded after PlayTrack = %q, want %q (reapplied)", player.last(), wantFilter)
	}
}

// TestEQNotPlayingApplyErrorStillSaves covers Fix 2 case (a): running an /eq
// command before playback, where the player isn't up so SetAudioFilter fails,
// must NOT surface an error and must still persist the EQ state (it will apply
// on the next track via the Facade's cached filter).
func TestEQNotPlayingApplyErrorStillSaves(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	player := &erroringEQPlayer{running: false, filterErr: fmt.Errorf("mpv not connected")}
	f := NewFacade(mouseSearcher{}, player, mouseStore{})
	app := NewApp(f, config.DefaultConfig())

	out, _, err := app.dispatch.Execute("/eq bass")
	if err != nil {
		t.Fatalf("/eq bass while not playing must not surface apply error, got: %v", err)
	}
	if !strings.Contains(strings.ToLower(out), "bass") {
		t.Errorf("feedback = %q, want it to mention bass", out)
	}
	// The player was still asked to apply the filter (which it rejected)...
	if player.last() == "" {
		t.Error("expected SetAudioFilter to still be attempted")
	}
	// ...and the state was persisted despite that failure.
	got := config.Load()
	if got.EQPreset != "bass" || !got.EQEnabled {
		t.Errorf("saved config = {preset:%q enabled:%v}, want {bass true}", got.EQPreset, got.EQEnabled)
	}
}

// TestEQPlayingApplyErrorSurfacesButStillSaves covers Fix 2 case (b): while a
// track is actually playing, a failed SetAudioFilter is a real error and must
// surface — but the EQ state is still saved first (persist-before-apply).
func TestEQPlayingApplyErrorSurfacesButStillSaves(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	player := &erroringEQPlayer{running: true, filterErr: fmt.Errorf("mpv af failed")}
	f := NewFacade(mouseSearcher{}, player, mouseStore{})
	app := NewApp(f, config.DefaultConfig())

	_, _, err := app.dispatch.Execute("/eq bass")
	if err == nil {
		t.Fatal("expected the apply error to surface while a track is playing")
	}
	// State must have been persisted before the apply was attempted.
	got := config.Load()
	if got.EQPreset != "bass" || !got.EQEnabled {
		t.Errorf("saved config = {preset:%q enabled:%v}, want {bass true} (must save before applying)", got.EQPreset, got.EQEnabled)
	}
}

// TestEQInitLoadsStoredGainsWhenDisabled covers Fix 4: a config carrying custom
// gains with eq_enabled=false must load those gains at Init (without applying a
// filter), so a later `/eq band` builds on the retained curve, not a flat one.
func TestEQInitLoadsStoredGainsWhenDisabled(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp)

	player := &eqPlayer{}
	f := NewFacade(mouseSearcher{}, player, mouseStore{})

	cfg := config.DefaultConfig()
	cfg.AutoUpdateYtDlp = false
	cfg.MediaKeys = false
	cfg.Volume = 0
	cfg.EQPreset = "custom"
	gains := make([]float64, 18)
	gains[2] = 5.0 // band 3 = +5 dB, retained but currently disabled
	cfg.EQGains = gains
	cfg.EQEnabled = false

	app := NewApp(f, cfg)
	app.Init()

	// Disabled EQ must not push a filter to the player at startup.
	if player.last() != "" {
		t.Errorf("filter applied at Init while EQ disabled = %q, want none", player.last())
	}

	if _, _, err := app.dispatch.Execute("/eq band 1 2"); err != nil {
		t.Fatalf("/eq band 1 2: %v", err)
	}

	filter := app.eq.FilterString()
	// band 3 = +5 dB → 10^(5/20) ≈ 1.7783 → "3b=1.8" (the RETAINED gain).
	if !strings.Contains(filter, "3b=1.8") {
		t.Errorf("filter %q lost the stored band-3 gain (want 3b=1.8)", filter)
	}
	// band 1 = +2 dB → 10^(2/20) ≈ 1.2589 → "1b=1.3" (the NEW gain).
	if !strings.Contains(filter, "1b=1.3") {
		t.Errorf("filter %q missing the new band-1 gain (want 1b=1.3)", filter)
	}
}
