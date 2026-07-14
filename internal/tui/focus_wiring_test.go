package tui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/config"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/core"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/tui/components"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/tui/focus"
)

// focusMarkers are substrings unique to each fake /focus screen kind (see
// internal/tui/focus.Render's htop/build-log/test-runner outputs). They are
// only safe as an ABSENCE check (the normal player UI never contains any of
// them): presence is NOT guaranteed for every kind at tick 0 — the
// test-runner screen reveals only its pytest session header at first, with
// no "PASSED" line yet — so activation tests must pin app.focusKind to a
// known kind instead of asserting "any marker".
var focusMarkers = []string{"Load average", "Compiling", "PASSED"}

const focusTestTrackTitle = "My Distinctive Focus Test Track"

// newFocusTestApp builds an App with a track already playing (a distinctive
// title, so tests can assert the /focus overlay hides it) and a window size
// set so View() renders real content.
func newFocusTestApp(t *testing.T) *App {
	t.Helper()
	player := &mousePlayer{}
	f := NewFacade(mouseSearcher{}, player, mouseStore{})

	if err := f.PlayTrack(context.Background(), core.Track{
		VideoID:  "v",
		Title:    focusTestTrackTitle,
		Duration: 100 * time.Second,
	}); err != nil {
		t.Fatalf("PlayTrack: %v", err)
	}

	app := NewApp(f, config.DefaultConfig())
	app.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	app.currentView = viewNowPlaying

	return app
}

// activateFocus drives /focus through the real submit path (dispatcher +
// handleSubmit sentinel mapping), the same way a user typing "/focus" and
// pressing Enter would.
func activateFocus(t *testing.T, app *App) tea.Cmd {
	t.Helper()
	_, cmd := app.Update(components.PromptSubmitMsg{Value: "/focus"})
	if !app.focusActive {
		t.Fatal("expected /focus to activate the overlay (focusActive = true)")
	}
	return cmd
}

func containsAnyMarker(s string) bool {
	for _, m := range focusMarkers {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}

func TestFocusSubmitActivatesOverlayAndHidesTrackTitle(t *testing.T) {
	app := newFocusTestApp(t)

	activateFocus(t, app)

	// /focus picks a RANDOM kind with a time-based seed, and not every
	// kind shows a stable marker at tick 0 (the test-runner screen's
	// first 8 revealed lines are pytest session header only — no
	// "PASSED" yet), which made a marker-for-any-kind assertion flake.
	// Pin the kind/seed to htop, which renders its full screen —
	// including "Load average" — on every tick, so the assertion is
	// deterministic while still exercising the real View() overlay path.
	app.focusKind = focus.KindHtop
	app.focusSeed = 42

	view := app.View()
	if !strings.Contains(view, "Load average") {
		t.Errorf("View() = %q, want the pinned htop overlay marker \"Load average\"", view)
	}
	if strings.Contains(view, focusTestTrackTitle) {
		t.Errorf("View() = %q, should not leak the track title while focus is active", view)
	}
}

func TestFocusAnyKeyDismisses(t *testing.T) {
	app := newFocusTestApp(t)
	activateFocus(t, app)

	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})

	if app.focusActive {
		t.Fatal("expected any non-ctrl+c key to dismiss the focus overlay")
	}

	view := app.View()
	if containsAnyMarker(view) {
		t.Errorf("View() = %q, still shows the fake overlay after dismissal", view)
	}
	if !strings.Contains(view, focusTestTrackTitle) {
		t.Errorf("View() = %q, want the normal now-playing view (with track title) restored", view)
	}
}

func TestFocusCtrlCQuits(t *testing.T) {
	app := newFocusTestApp(t)
	activateFocus(t, app)

	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected ctrl+c while focus is active to return a quit cmd")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf("expected cmd() to yield tea.QuitMsg, got %T", cmd())
	}
}

func TestFocusMouseSwallowedWhileActive(t *testing.T) {
	app := newFocusTestApp(t)
	activateFocus(t, app)

	_, cmd := app.Update(tea.MouseMsg{
		X:      10,
		Y:      10,
		Action: tea.MouseActionPress,
		Button: tea.MouseButtonLeft,
	})

	if !app.focusActive {
		t.Fatal("expected focus overlay to remain active after a mouse event")
	}
	if cmd != nil {
		t.Errorf("expected mouse events to be swallowed (nil cmd) while focus is active, got %v", cmd)
	}
}

func TestFocusTickIncrementsAndReschedulesWhileActive(t *testing.T) {
	app := newFocusTestApp(t)
	activateFocus(t, app)

	_, cmd := app.Update(focusTickMsg{gen: app.focusGen})

	if app.focusTick != 1 {
		t.Errorf("focusTick = %d, want 1", app.focusTick)
	}
	if cmd == nil {
		t.Fatal("expected another tick cmd to be scheduled while focus is active")
	}
}

func TestFocusTickNoopWhileInactive(t *testing.T) {
	app := newFocusTestApp(t)
	// Note: focus is never activated in this test.

	before := app.focusTick
	_, cmd := app.Update(focusTickMsg{gen: app.focusGen})

	if app.focusTick != before {
		t.Errorf("focusTick changed from %d to %d on a stray tick while inactive", before, app.focusTick)
	}
	if cmd != nil {
		t.Errorf("expected no cmd from a stray focusTickMsg while inactive, got %v", cmd)
	}
}

// TestFocusStaleTickFromPreviousSessionDropped reproduces the reviewer's
// tick-chain duplication timeline: /focus (chain A scheduled) → dismiss
// before A's tick fires → /focus again (chain B scheduled) → chain A's
// stale tick finally arrives. With only a focusActive boolean check, the
// stale tick sees focusActive==true, increments, AND reschedules — leaving
// two self-perpetuating 2s chains advancing focusTick at double speed.
// The generation guard must drop it: no increment, no reschedule.
func TestFocusStaleTickFromPreviousSessionDropped(t *testing.T) {
	app := newFocusTestApp(t)

	// Session A: activate, capture its generation.
	activateFocus(t, app)
	staleGen := app.focusGen

	// Dismiss before session A's in-flight tick arrives.
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if app.focusActive {
		t.Fatal("setup: expected dismissal to deactivate focus")
	}

	// Session B: reactivate; its own tick chain is now scheduled.
	activateFocus(t, app)
	if app.focusGen == staleGen {
		t.Fatal("setup: expected reactivation to bump focusGen")
	}

	// Session A's stale tick arrives late.
	_, cmd := app.Update(focusTickMsg{gen: staleGen})

	if app.focusTick != 0 {
		t.Errorf("focusTick = %d after a stale-gen tick, want 0 (dropped, not double-advanced)", app.focusTick)
	}
	if cmd != nil {
		t.Errorf("stale-gen tick must not reschedule (would fork a second 2s chain), got cmd %v", cmd)
	}

	// A current-gen tick still drives session B's animation normally.
	_, cmd = app.Update(focusTickMsg{gen: app.focusGen})
	if app.focusTick != 1 {
		t.Errorf("focusTick = %d after a current-gen tick, want 1", app.focusTick)
	}
	if cmd == nil {
		t.Error("expected the current-gen tick to reschedule the next frame")
	}
}
