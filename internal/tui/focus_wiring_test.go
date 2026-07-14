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
)

// focusMarkers are substrings unique to each fake /focus screen kind (see
// internal/tui/focus.Render's htop/build-log/test-runner outputs). A view
// containing any one of these is displaying the overlay, not the normal
// player UI.
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

	view := app.View()
	if !containsAnyMarker(view) {
		t.Errorf("View() = %q, want it to contain one of the fake markers %v", view, focusMarkers)
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

	_, cmd := app.Update(focusTickMsg{})

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
	_, cmd := app.Update(focusTickMsg{})

	if app.focusTick != before {
		t.Errorf("focusTick changed from %d to %d on a stray tick while inactive", before, app.focusTick)
	}
	if cmd != nil {
		t.Errorf("expected no cmd from a stray focusTickMsg while inactive, got %v", cmd)
	}
}
