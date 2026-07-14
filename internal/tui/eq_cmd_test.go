package tui

import (
	"context"
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
