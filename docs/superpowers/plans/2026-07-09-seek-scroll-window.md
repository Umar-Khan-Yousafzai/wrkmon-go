# Seek Suite + Infinite-Scroll Search + App Window — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add full seek control (keys, `/seek`, clickable seekbar), infinite-scroll search results, and a `wrkmon-go window` mode that launches the TUI in its own desktop window.

**Architecture:** Three independent feature slices over the existing hexagonal layout: seek extends the `ports.Player` interface down into the mpv adapter and up into the TUI; infinite scroll adds pure accumulate/paging helpers in `core` plus TUI wiring; the window mode is a new `internal/window` package that re-execs the binary inside a terminal emulator in app mode.

**Tech Stack:** Go, Bubble Tea v1.3.10 (mouse API: `tea.MouseMsg{Action, Button, X, Y}`), lipgloss v1.1.0, mpv JSON IPC, yt-dlp subprocess.

**Spec:** `docs/superpowers/specs/2026-07-09-seek-scroll-window-design.md`

## Global Constraints

- Module path: `github.com/Umar-Khan-Yousafzai/wrkmon-go`
- Zero CGO (`CGO_ENABLED=0` builds must keep working)
- Run tests with `go test ./...` from repo root; all must pass before every commit
- Run `gofmt -l .` before every commit; output must be empty
- Commit messages: conventional commits, NO Co-Authored-By or AI attribution lines
- Bubble Tea v1 mouse API only (`msg.Action`/`msg.Button`, NOT the deprecated `msg.Type`)
- The `--class`/`--app-id` value is exactly `wrkmon-go` everywhere (terminals, desktop file `StartupWMClass`)
- Config defaults: `mouse = true`, `max_search_results = 100`, `[window] terminal = "auto"`
- CROSS-PLATFORM IS MANDATORY: every feature must work on Linux, macOS, and Windows. OS-specific behavior goes behind `runtime.GOOS` switches or `_windows.go`/`_unix.go` files, never breaks the other platforms' builds. Seek/scroll/mouse are pure Bubble Tea (all OSes); window mode has per-OS launch paths (Linux terminal table, Windows `wt`→`cmd start`, macOS brew terminals→AppleScript); desktop integration ships per-OS (.desktop, Start-Menu .lnk, ~/Applications .app bundle)

---

### Task 1: `SeekTo` / `SeekPercent` on the Player port, mpv adapter, and facade

**Files:**
- Modify: `internal/ports/ports.go` (Player interface, after line 22 `Seek(...)`)
- Modify: `internal/adapters/mpv/mpv.go` (after `Seek`, ~line 168)
- Modify: `internal/tui/facade.go` (after `Seek`, ~line 160)
- Test: `internal/adapters/mpv/mpv_test.go`

**Interfaces:**
- Consumes: existing `(m *MPV) command(...)` IPC helper, `ports.Player`
- Produces: `Player.SeekTo(seconds float64) error`, `Player.SeekPercent(pct float64) error`, `(f *Facade) SeekTo(seconds float64) error`, `(f *Facade) SeekPercent(pct float64) error` — Tasks 3, 5, 6 call the facade methods.

- [ ] **Step 1: Write the failing tests** — append to `internal/adapters/mpv/mpv_test.go`:

```go
func TestSeekToNotConnected(t *testing.T) {
	m := &MPV{}
	if err := m.SeekTo(30); err == nil {
		t.Error("expected error when SeekTo called without connection")
	}
}

func TestSeekPercentNotConnected(t *testing.T) {
	m := &MPV{}
	if err := m.SeekPercent(50); err == nil {
		t.Error("expected error when SeekPercent called without connection")
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/adapters/mpv/ -run TestSeek -v`
Expected: compile error — `m.SeekTo undefined`

- [ ] **Step 3: Implement**

`internal/ports/ports.go` — inside the `Player` interface, directly under `Seek(seconds float64) error`:

```go
	SeekTo(seconds float64) error
	SeekPercent(pct float64) error
```

`internal/adapters/mpv/mpv.go` — directly under the existing `Seek` method:

```go
// SeekTo seeks to an absolute position in seconds.
func (m *MPV) SeekTo(seconds float64) error {
	_, err := m.command("seek", seconds, "absolute")
	return err
}

// SeekPercent seeks to an absolute percentage (0–100) of the duration.
func (m *MPV) SeekPercent(pct float64) error {
	_, err := m.command("seek", pct, "absolute-percent")
	return err
}
```

`internal/tui/facade.go` — directly under the existing `Seek` method:

```go
// SeekTo seeks to an absolute position in seconds.
func (f *Facade) SeekTo(seconds float64) error {
	return f.player.SeekTo(seconds)
}

// SeekPercent seeks to a percentage (0–100) of the track duration.
func (f *Facade) SeekPercent(pct float64) error {
	return f.player.SeekPercent(pct)
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./... `
Expected: all PASS (the compile-time `var _ ports.Player = (*MPV)(nil)` check in mpv_test.go also proves interface conformance)

- [ ] **Step 5: Commit**

```bash
git add internal/ports/ports.go internal/adapters/mpv/mpv.go internal/adapters/mpv/mpv_test.go internal/tui/facade.go
git commit -m "feat(player): add absolute SeekTo and SeekPercent to Player port"
```

---

### Task 2: `ParseSeek` — pure parser for /seek arguments

**Files:**
- Create: `internal/tui/commands/seek.go`
- Test: `internal/tui/commands/seek_test.go`

**Interfaces:**
- Produces: `commands.SeekKind` (`SeekAbsolute`, `SeekPct`, `SeekRelative`), `commands.SeekSpec{Kind SeekKind; Value float64}`, `commands.ParseSeek(arg string) (SeekSpec, error)` — Task 3 consumes.

- [ ] **Step 1: Write the failing test** — `internal/tui/commands/seek_test.go`:

```go
package commands

import "testing"

func TestParseSeek(t *testing.T) {
	cases := []struct {
		in      string
		want    SeekSpec
		wantErr bool
	}{
		{"1:23", SeekSpec{SeekAbsolute, 83}, false},
		{"01:02:03", SeekSpec{SeekAbsolute, 3723}, false},
		{"0:00", SeekSpec{SeekAbsolute, 0}, false},
		{"83", SeekSpec{SeekAbsolute, 83}, false},
		{"83.5", SeekSpec{SeekAbsolute, 83.5}, false},
		{"50%", SeekSpec{SeekPct, 50}, false},
		{"0%", SeekSpec{SeekPct, 0}, false},
		{"100%", SeekSpec{SeekPct, 100}, false},
		{"+30", SeekSpec{SeekRelative, 30}, false},
		{"-30", SeekSpec{SeekRelative, -30}, false},
		{"", SeekSpec{}, true},
		{"abc", SeekSpec{}, true},
		{"1:99", SeekSpec{}, true},   // seconds field must be < 60
		{"1:2:3:4", SeekSpec{}, true}, // too many colon fields
		{"150%", SeekSpec{}, true},   // percent must be 0–100
		{"-1:00", SeekSpec{}, true},  // sign + colon form not allowed
		{"-5", SeekSpec{SeekRelative, -5}, false},
	}
	for _, c := range cases {
		got, err := ParseSeek(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseSeek(%q): expected error, got %+v", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseSeek(%q): unexpected error %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("ParseSeek(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/commands/ -run TestParseSeek -v`
Expected: compile error — `undefined: SeekSpec`

- [ ] **Step 3: Implement** — `internal/tui/commands/seek.go`:

```go
package commands

import (
	"fmt"
	"strconv"
	"strings"
)

// SeekKind classifies a parsed /seek argument.
type SeekKind int

const (
	SeekAbsolute SeekKind = iota // seconds from track start
	SeekPct                      // percent of duration, 0–100
	SeekRelative                 // seconds from current position (signed)
)

// SeekSpec is a parsed /seek argument.
type SeekSpec struct {
	Kind  SeekKind
	Value float64
}

// ParseSeek parses a /seek argument. Accepted forms:
//
//	1:23  01:02:03   absolute mm:ss or h:mm:ss
//	83  83.5          absolute seconds
//	50%               percent of duration (0–100)
//	+30  -30          relative seconds
func ParseSeek(arg string) (SeekSpec, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return SeekSpec{}, fmt.Errorf("empty seek target")
	}

	// Relative: leading sign.
	if arg[0] == '+' || arg[0] == '-' {
		v, err := strconv.ParseFloat(arg, 64)
		if err != nil {
			return SeekSpec{}, fmt.Errorf("bad relative offset %q", arg)
		}
		return SeekSpec{SeekRelative, v}, nil
	}

	// Percent: trailing %.
	if strings.HasSuffix(arg, "%") {
		v, err := strconv.ParseFloat(strings.TrimSuffix(arg, "%"), 64)
		if err != nil || v < 0 || v > 100 {
			return SeekSpec{}, fmt.Errorf("percent must be 0–100")
		}
		return SeekSpec{SeekPct, v}, nil
	}

	// Timestamp: contains a colon.
	if strings.Contains(arg, ":") {
		parts := strings.Split(arg, ":")
		if len(parts) < 2 || len(parts) > 3 {
			return SeekSpec{}, fmt.Errorf("timestamp must be mm:ss or h:mm:ss")
		}
		total := 0
		for i, p := range parts {
			n, err := strconv.Atoi(p)
			if err != nil || n < 0 {
				return SeekSpec{}, fmt.Errorf("bad timestamp %q", arg)
			}
			// minutes/seconds fields (all but the first) must be < 60
			if i > 0 && n > 59 {
				return SeekSpec{}, fmt.Errorf("bad timestamp %q", arg)
			}
			total = total*60 + n
		}
		return SeekSpec{SeekAbsolute, float64(total)}, nil
	}

	// Plain seconds.
	v, err := strconv.ParseFloat(arg, 64)
	if err != nil || v < 0 {
		return SeekSpec{}, fmt.Errorf("bad seek target %q", arg)
	}
	return SeekSpec{SeekAbsolute, v}, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui/commands/ -v`
Expected: PASS (including pre-existing dispatcher tests)

- [ ] **Step 5: Commit**

```bash
git add internal/tui/commands/seek.go internal/tui/commands/seek_test.go
git commit -m "feat(tui): ParseSeek parser for /seek arguments"
```

---

### Task 3: `/seek` command, shift+arrow jumps, seek toasts, help + first-play hint

**Files:**
- Modify: `internal/tui/app.go` — `App` struct (~line 107), `buildCommands` (~line 127), help text (~line 137), key switch in `Update` (~line 439), `PlaybackStartedMsg` handler (~line 583)

**Interfaces:**
- Consumes: `commands.ParseSeek` (Task 2), `Facade.SeekTo/SeekPercent/Seek` (Task 1), existing `formatSeconds(float64) string` helper, existing toast pattern `a.toast.Show(text, isErr) tea.Cmd`
- Produces: `(a *App) seekRelativeKey(delta float64) tea.Cmd` — Task 5/6 reuse the toast pattern but not this function.

Note: slash-command handlers are closures over `a` that mutate state directly and return a string; `handleSubmit` (~line 1075) toasts non-empty, non-sentinel return values. `/seek`'s returned string surfaces as a toast with no extra wiring.

- [ ] **Step 1: Add struct field** — in the `App` struct after `helpText string`:

```go
	// One-time seek hint, shown on first playback of the session
	seekHintShown bool
```

- [ ] **Step 2: Add `execSeek` and `seekRelativeKey` helpers** — near `moveCursorUp` (~line 1205):

```go
// execSeek applies a parsed /seek argument and returns the toast text.
func (a *App) execSeek(args string) (string, error) {
	if a.facade.State().Current == nil {
		return "", fmt.Errorf("nothing playing")
	}
	spec, err := commands.ParseSeek(args)
	if err != nil {
		return "", fmt.Errorf("usage: /seek 1:23 | 83 | 50%% | +30 | -30 (%v)", err)
	}
	var target float64
	switch spec.Kind {
	case commands.SeekAbsolute:
		target = spec.Value
		err = a.facade.SeekTo(spec.Value)
	case commands.SeekPct:
		target = spec.Value / 100 * a.currentDur
		err = a.facade.SeekPercent(spec.Value)
	case commands.SeekRelative:
		target = a.currentPos + spec.Value
		err = a.facade.Seek(spec.Value)
	}
	if err != nil {
		return "", err
	}
	a.currentPos = clampPos(target, a.currentDur)
	return fmt.Sprintf("⏩ %s / %s", formatSeconds(a.currentPos), formatSeconds(a.currentDur)), nil
}

// seekRelativeKey seeks by delta seconds and returns a toast command.
func (a *App) seekRelativeKey(delta float64) tea.Cmd {
	if a.facade.State().Current == nil {
		return nil
	}
	if err := a.facade.Seek(delta); err != nil {
		return a.toast.Show(err.Error(), true)
	}
	a.currentPos = clampPos(a.currentPos+delta, a.currentDur)
	return a.toast.Show(fmt.Sprintf("⏩ %s / %s", formatSeconds(a.currentPos), formatSeconds(a.currentDur)), false)
}

// clampPos clamps a seek target into [0, dur] (dur 0 = unknown, only floor).
func clampPos(pos, dur float64) float64 {
	if pos < 0 {
		return 0
	}
	if dur > 0 && pos > dur {
		return dur
	}
	return pos
}
```

- [ ] **Step 3: Register `/seek`** — in `buildCommands`, after the `/vol` registration:

```go
	d.Register(commands.Command{
		Name:        "/seek",
		Description: "Seek: /seek 1:23 | 83 | 50% | +30 | -30",
		Handler: func(args string) (string, error) {
			return a.execSeek(args)
		},
	})
```

- [ ] **Step 4: Rewire arrow keys** — in the `Update` key switch, replace the existing `left`/`right` cases (~lines 439–449):

```go
		case "left":
			// Seek backward 5s when prompt is empty
			if a.prompt.Value() == "" {
				cmds = append(cmds, a.seekRelativeKey(-5))
				return a, tea.Batch(cmds...)
			}
		case "right":
			// Seek forward 5s when prompt is empty
			if a.prompt.Value() == "" {
				cmds = append(cmds, a.seekRelativeKey(5))
				return a, tea.Batch(cmds...)
			}
		case "shift+left":
			cmds = append(cmds, a.seekRelativeKey(-30))
			return a, tea.Batch(cmds...)
		case "shift+right":
			cmds = append(cmds, a.seekRelativeKey(30))
			return a, tea.Batch(cmds...)
```

- [ ] **Step 5: Update help text** — in `buildCommands`'s `/help` handler, replace the line `"  Left/Right     Seek -/+ 5 seconds\n" +` with:

```go
				"  Left/Right     Seek -/+ 5 seconds\n" +
				"  Shift+←/→      Seek -/+ 30 seconds\n" +
```

- [ ] **Step 6: First-play hint** — in the `PlaybackStartedMsg` handler (~line 583), after the existing `Now playing` toast append:

```go
		if !a.seekHintShown {
			a.seekHintShown = true
			cmds = append(cmds, a.toast.Show("Seek: ←/→ 5s · Shift ±30s · /seek 1:23 · click the bar", false))
		}
```

(The hint toast replaces the now-playing toast — acceptable, it appears once per run.)

- [ ] **Step 7: Build + test + manual check**

Run: `go build ./... && go test ./...`
Expected: build OK, all tests PASS.
Manual: `go run ./cmd/wrkmon-go`, play a track, verify: hint toast appears once; `→` toasts `⏩ 0:05 / …`; `shift+→` jumps 30s even while the prompt has text; `/seek 50%` lands mid-track; `/seek xyz` toasts the usage line.

- [ ] **Step 8: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat(tui): /seek command, shift+arrow 30s jumps, seek toasts, first-play hint"
```

---

### Task 4: Config — `mouse`, `max_search_results`

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go` (new)

**Interfaces:**
- Produces: `Config.Mouse bool` (default true), `Config.MaxSearchResults int` (default 100) — Tasks 5 and 9 consume. (`Config.Window` is added in Task 10 with its consumer.)

- [ ] **Step 1: Write the failing test** — `internal/config/config_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func setTempHome(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("USERPROFILE", tmp) // windows
	return tmp
}

func TestDefaults(t *testing.T) {
	setTempHome(t)
	cfg := Load() // no file on disk
	if !cfg.Mouse {
		t.Error("Mouse should default to true")
	}
	if cfg.MaxSearchResults != 100 {
		t.Errorf("MaxSearchResults = %d, want 100", cfg.MaxSearchResults)
	}
}

func TestLoadKeepsDefaultsForMissingKeys(t *testing.T) {
	tmp := setTempHome(t)
	dir := filepath.Join(tmp, ".config", "wrkmon-go")
	os.MkdirAll(dir, 0o755)
	// A pre-existing config that predates the new keys.
	os.WriteFile(filepath.Join(dir, "config.toml"), []byte("theme = \"github-dark\"\nvolume = 70\n"), 0o644)
	cfg := Load()
	if !cfg.Mouse {
		t.Error("Mouse should stay true when key absent")
	}
	if cfg.MaxSearchResults != 100 {
		t.Errorf("MaxSearchResults = %d, want 100", cfg.MaxSearchResults)
	}
	if cfg.Volume != 70 {
		t.Errorf("Volume = %d, want 70", cfg.Volume)
	}
}

func TestLoadHonorsExplicitValues(t *testing.T) {
	tmp := setTempHome(t)
	dir := filepath.Join(tmp, ".config", "wrkmon-go")
	os.MkdirAll(dir, 0o755)
	os.WriteFile(filepath.Join(dir, "config.toml"), []byte("mouse = false\nmax_search_results = 40\n"), 0o644)
	cfg := Load()
	if cfg.Mouse {
		t.Error("mouse = false should be honored")
	}
	if cfg.MaxSearchResults != 40 {
		t.Errorf("MaxSearchResults = %d, want 40", cfg.MaxSearchResults)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -v`
Expected: compile error — `cfg.Mouse undefined`

- [ ] **Step 3: Implement** — in `internal/config/config.go`, add to the `Config` struct:

```go
	Mouse            bool `toml:"mouse"`
	MaxSearchResults int  `toml:"max_search_results"`
```

and to `DefaultConfig()`:

```go
		Mouse:            true,
		MaxSearchResults: 100,
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): mouse and max_search_results keys"
```

---

### Task 5: Mouse mode + statusbar seekbar click/drag + wheel scroll

**Files:**
- Modify: `cmd/wrkmon-go/main.go` (~line 47)
- Modify: `internal/tui/components/statusbar.go` (factor bar layout, add `BarBounds`)
- Modify: `internal/tui/app.go` (`App` struct, `Update` — new `tea.MouseMsg` case)
- Test: `internal/tui/components/statusbar_test.go` (new)

**Interfaces:**
- Consumes: `Facade.SeekPercent` (Task 1), `Config.Mouse` (Task 4)
- Produces: `(s StatusBar) BarBounds() (start, width int, ok bool)`; `(a *App) seekToColumn(x, barStart, barWidth int) tea.Cmd` — Task 6 reuses `seekToColumn`.

- [ ] **Step 1: Write the failing test** — `internal/tui/components/statusbar_test.go`:

```go
package components

import (
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/core"
	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/tui/theme"
)

func playingBar(width int) StatusBar {
	s := NewStatusBar(theme.Get("opencode-mono"))
	s.SetWidth(width)
	s.SetState(core.PlayerState{
		Status:  core.StatusPlaying,
		Current: &core.Track{Title: "Test Song", Duration: 100 * time.Second},
		Volume:  50,
	})
	s.SetPosition(10, 100)
	return s
}

func TestBarBoundsPlaying(t *testing.T) {
	s := playingBar(100)
	start, width, ok := s.BarBounds()
	if !ok {
		t.Fatal("expected a bar at width 100 while playing")
	}
	if width <= 4 {
		t.Errorf("bar width = %d, want > 4", width)
	}
	if start <= 0 || start+width >= 100 {
		t.Errorf("bar [%d, %d) out of range for width 100", start, start+width)
	}
	// The bar must sit within the rendered line.
	if lipgloss.Width(s.View()) < start+width {
		t.Error("bar bounds exceed rendered width")
	}
}

func TestBarBoundsStopped(t *testing.T) {
	s := NewStatusBar(theme.Get("opencode-mono"))
	s.SetWidth(100)
	if _, _, ok := s.BarBounds(); ok {
		t.Error("no bar expected when nothing is playing")
	}
}

func TestBarBoundsNarrow(t *testing.T) {
	s := playingBar(30) // too narrow for the mini bar (gap <= 4)
	if _, _, ok := s.BarBounds(); ok {
		t.Error("no bar expected on a narrow status bar")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/components/ -v`
Expected: compile error — `s.BarBounds undefined`

- [ ] **Step 3: Refactor statusbar** — in `internal/tui/components/statusbar.go`, replace the body of `View()` and add `barLayout`/`BarBounds`. The layout math moves verbatim into `barLayout`; `View` becomes a consumer:

```go
// barLayout computes the rendered pieces and the mini-bar geometry.
// ok is false when the bar is not drawn (stopped, or too narrow).
func (s StatusBar) barLayout() (left, right string, barStart, barWidth int, ok bool) {
	if s.state.Current != nil {
		status := "▶"
		if s.state.Status == core.StatusPaused {
			status = "⏸"
		}
		posStr := fmtSecs(s.position)
		durStr := fmtSecs(s.duration)
		left = fmt.Sprintf(" %s %s  %s/%s", status, truncate(s.state.Current.Title, 40), posStr, durStr)
	} else {
		left = " ■ Stopped"
	}

	modes := ""
	if s.repeatMode != "" && s.repeatMode != "off" {
		modes += " ↻" + s.repeatMode
	}
	if s.shuffle {
		modes += " ⇄"
	}
	right = fmt.Sprintf("Vol: %d%%%s │ %s ", s.state.Volume, modes, s.view)

	gap := s.width - lipgloss.Width(left) - lipgloss.Width(right) - 2
	if gap > 4 && s.state.Current != nil {
		return left, right, lipgloss.Width(left) + 1, gap, true
	}
	return left, right, 0, 0, false
}

// BarBounds returns the column range [start, start+width) of the mini
// progress bar on the status row, or ok=false if no bar is drawn.
func (s StatusBar) BarBounds() (start, width int, ok bool) {
	_, _, start, width, ok = s.barLayout()
	return
}

// View renders the status bar.
func (s StatusBar) View() string {
	if s.width <= 0 {
		return ""
	}
	left, right, _, gap, ok := s.barLayout()
	if ok {
		bar := miniBar(s.position, s.duration, gap)
		return s.styles.StatusBar.Width(s.width).Render(left + " " + bar + " " + right)
	}
	pad := s.width - lipgloss.Width(left) - lipgloss.Width(right)
	if pad < 0 {
		pad = 0
	}
	return s.styles.StatusBar.Width(s.width).Render(left + strings.Repeat(" ", pad) + right)
}
```

(Delete the old duplicated layout code from `View`; `miniBar`, `truncate`, `fmtSecs` stay.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui/components/ -v`
Expected: PASS

- [ ] **Step 5: Enable mouse in main** — in `cmd/wrkmon-go/main.go`, replace `p := tea.NewProgram(app, tea.WithAltScreen())` with:

```go
	opts := []tea.ProgramOption{tea.WithAltScreen()}
	if cfg.Mouse {
		opts = append(opts, tea.WithMouseCellMotion())
	}
	p := tea.NewProgram(app, opts...)
```

- [ ] **Step 6: Handle mouse in the TUI** — in `internal/tui/app.go`:

Add fields to the `App` struct after `seekHintShown bool`:

```go
	// Seekbar drag state
	seekDragging bool
	lastDragSeek time.Time
```

(`time` is already imported in app.go, line 7.)

Add a new top-level case to `Update`'s `switch msg := msg.(type)`, next to `case tea.KeyMsg:`:

```go
	case tea.MouseMsg:
		if cmd := a.handleMouse(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return a, tea.Batch(cmds...)
```

Add the handler + shared seek helper near `moveCursorUp`:

```go
// handleMouse routes wheel scrolling and seekbar clicks/drags.
func (a *App) handleMouse(msg tea.MouseMsg) tea.Cmd {
	// Wheel: scroll the focused list.
	if msg.Button == tea.MouseButtonWheelUp && msg.Action == tea.MouseActionPress {
		a.moveCursorUp()
		return nil
	}
	if msg.Button == tea.MouseButtonWheelDown && msg.Action == tea.MouseActionPress {
		a.moveCursorDown()
		return nil
	}

	// Statusbar seekbar. The status row sits directly above the prompt.
	statusRow := a.height - lipgloss.Height(a.prompt.View()) - 1
	barStart, barWidth, ok := a.statusBar.BarBounds()
	onBar := ok && msg.Y == statusRow && msg.X >= barStart && msg.X < barStart+barWidth

	switch {
	case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft && onBar:
		a.seekDragging = true
		return a.seekToColumn(msg.X, barStart, barWidth)
	case msg.Action == tea.MouseActionMotion && a.seekDragging:
		if time.Since(a.lastDragSeek) >= 250*time.Millisecond {
			return a.seekToColumn(msg.X, barStart, barWidth)
		}
	case msg.Action == tea.MouseActionRelease && a.seekDragging:
		a.seekDragging = false
		return a.seekToColumn(msg.X, barStart, barWidth)
	}
	return nil
}

// seekToColumn seeks to the position that column x represents on a bar
// spanning [barStart, barStart+barWidth).
func (a *App) seekToColumn(x, barStart, barWidth int) tea.Cmd {
	if barWidth <= 0 || a.facade.State().Current == nil {
		return nil
	}
	col := x - barStart
	if col < 0 {
		col = 0
	}
	if col >= barWidth {
		col = barWidth - 1
	}
	pct := float64(col) / float64(barWidth-1) * 100
	if err := a.facade.SeekPercent(pct); err != nil {
		return a.toast.Show(err.Error(), true)
	}
	a.lastDragSeek = time.Now()
	a.currentPos = clampPos(pct/100*a.currentDur, a.currentDur)
	return a.toast.Show(fmt.Sprintf("⏩ %s / %s", formatSeconds(a.currentPos), formatSeconds(a.currentDur)), false)
}
```

- [ ] **Step 7: Build + test + manual check**

Run: `go build ./... && go test ./...`
Expected: PASS.
Manual: run the app, play a track, click mid-bar in the statusbar → position jumps and toast shows; hold and drag → position follows; wheel over search results moves the cursor; set `mouse = false` in config → clicks do nothing and terminal text selection works again.

- [ ] **Step 8: Commit**

```bash
git add cmd/wrkmon-go/main.go internal/tui/components/statusbar.go internal/tui/components/statusbar_test.go internal/tui/app.go
git commit -m "feat(tui): mouse mode with clickable/draggable statusbar seekbar and wheel scroll"
```

---

### Task 6: Now-playing view — clickable big progress bar

**Files:**
- Modify: `internal/tui/app.go` (`App` struct, `renderNowPlayingView` ~line 920, `handleMouse`)

**Interfaces:**
- Consumes: `seekToColumn` (Task 5)
- Produces: nothing new for later tasks.

- [ ] **Step 1: Record bar geometry during render** — add fields to the `App` struct after `lastDragSeek time.Time`:

```go
	// Now-playing big bar geometry, captured at render time
	npBarRow   int
	npBarStart int
	npBarWidth int
```

In `renderNowPlayingView` (pointer receiver — mutations persist): at the top of the "nothing playing" early return, invalidate:

```go
	if state.Current == nil {
		a.npBarWidth = 0
		return a.styles.Muted.Render("  Nothing playing.\n\n  Search for music to get started.")
	}
```

Immediately BEFORE the line `b.WriteString("  " + a.styles.Accent.Render(bar))`:

```go
	a.npBarRow = strings.Count(b.String(), "\n") // row index of the bar line
	a.npBarStart = 2
	a.npBarWidth = barWidth
```

- [ ] **Step 2: Route clicks** — in `handleMouse` (Task 5), extend the `onBar` logic. After computing `onBar`, add:

```go
	onNpBar := a.currentView == viewNowPlaying && a.helpText == "" && a.lyricsText == "" &&
		a.npBarWidth > 0 && msg.Y == a.npBarRow &&
		msg.X >= a.npBarStart && msg.X < a.npBarStart+a.npBarWidth
```

and change the press case to try either bar (drag keeps using whichever bar started it — capture geometry at press):

```go
	switch {
	case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft && onBar:
		a.seekDragging = true
		a.dragStart, a.dragWidth = barStart, barWidth
		return a.seekToColumn(msg.X, barStart, barWidth)
	case msg.Action == tea.MouseActionPress && msg.Button == tea.MouseButtonLeft && onNpBar:
		a.seekDragging = true
		a.dragStart, a.dragWidth = a.npBarStart, a.npBarWidth
		return a.seekToColumn(msg.X, a.npBarStart, a.npBarWidth)
	case msg.Action == tea.MouseActionMotion && a.seekDragging:
		if time.Since(a.lastDragSeek) >= 250*time.Millisecond {
			return a.seekToColumn(msg.X, a.dragStart, a.dragWidth)
		}
	case msg.Action == tea.MouseActionRelease && a.seekDragging:
		a.seekDragging = false
		return a.seekToColumn(msg.X, a.dragStart, a.dragWidth)
	}
```

Add the two capture fields to the `App` struct next to `seekDragging`:

```go
	dragStart int
	dragWidth int
```

- [ ] **Step 3: Build + test + manual check**

Run: `go build ./... && go test ./...`
Expected: PASS.
Manual: `/now`, click the big bar → seeks; drag → follows; click the same X position twice → same target (stable math).

- [ ] **Step 4: Commit**

```bash
git add internal/tui/app.go
git commit -m "feat(tui): clickable/draggable progress bar in now-playing view"
```

---

### Task 7: Pure paging helpers in core — `MergeResults`, `NextFetchSize`, `VisibleRange`

**Files:**
- Create: `internal/core/paging.go`
- Test: `internal/core/paging_test.go`

**Interfaces:**
- Produces: `core.MergeResults(existing, incoming []SearchResult) ([]SearchResult, int)`, `core.NextFetchSize(current, batch, max int) int`, `core.VisibleRange(cursor, total, height int) (start, end int)` — Task 9 consumes all three.

- [ ] **Step 1: Write the failing test** — `internal/core/paging_test.go`:

```go
package core

import "testing"

func sr(ids ...string) []SearchResult {
	out := make([]SearchResult, len(ids))
	for i, id := range ids {
		out[i] = SearchResult{VideoID: id, Title: "t-" + id}
	}
	return out
}

func TestMergeResults(t *testing.T) {
	merged, added := MergeResults(sr("a", "b"), sr("b", "c", "d"))
	if added != 2 {
		t.Errorf("added = %d, want 2", added)
	}
	if len(merged) != 4 {
		t.Fatalf("len = %d, want 4", len(merged))
	}
	want := []string{"a", "b", "c", "d"}
	for i, w := range want {
		if merged[i].VideoID != w {
			t.Errorf("merged[%d] = %s, want %s", i, merged[i].VideoID, w)
		}
	}
}

func TestMergeResultsAllDuplicates(t *testing.T) {
	merged, added := MergeResults(sr("a", "b"), sr("a", "b"))
	if added != 0 || len(merged) != 2 {
		t.Errorf("added=%d len=%d, want 0 and 2", added, len(merged))
	}
}

func TestMergeResultsEmptyExisting(t *testing.T) {
	merged, added := MergeResults(nil, sr("a"))
	if added != 1 || len(merged) != 1 {
		t.Errorf("added=%d len=%d, want 1 and 1", added, len(merged))
	}
}

func TestNextFetchSize(t *testing.T) {
	cases := []struct{ cur, batch, max, want int }{
		{10, 25, 100, 35},
		{85, 25, 100, 100}, // clamped to cap
		{100, 25, 100, 0},  // at cap
		{120, 25, 100, 0},  // beyond cap
		{0, 25, 100, 25},
	}
	for _, c := range cases {
		if got := NextFetchSize(c.cur, c.batch, c.max); got != c.want {
			t.Errorf("NextFetchSize(%d,%d,%d) = %d, want %d", c.cur, c.batch, c.max, got, c.want)
		}
	}
}

func TestVisibleRange(t *testing.T) {
	cases := []struct{ cursor, total, height, wantStart, wantEnd int }{
		{0, 5, 10, 0, 5},    // everything fits
		{0, 100, 10, 0, 10}, // top
		{50, 100, 10, 45, 55}, // centered on cursor
		{99, 100, 10, 90, 100}, // bottom clamp
		{3, 100, 10, 0, 10},  // near top clamp
		{0, 0, 10, 0, 0},     // empty
		{0, 5, 0, 0, 0},      // degenerate height
	}
	for _, c := range cases {
		s, e := VisibleRange(c.cursor, c.total, c.height)
		if s != c.wantStart || e != c.wantEnd {
			t.Errorf("VisibleRange(%d,%d,%d) = [%d,%d), want [%d,%d)",
				c.cursor, c.total, c.height, s, e, c.wantStart, c.wantEnd)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/core/ -run 'TestMerge|TestNext|TestVisible' -v`
Expected: compile error — `undefined: MergeResults`

- [ ] **Step 3: Implement** — `internal/core/paging.go`:

```go
package core

// MergeResults appends items of incoming whose VideoID is not already in
// existing, preserving order. Returns the merged slice and the count added.
func MergeResults(existing, incoming []SearchResult) ([]SearchResult, int) {
	seen := make(map[string]struct{}, len(existing))
	for _, r := range existing {
		seen[r.VideoID] = struct{}{}
	}
	added := 0
	for _, r := range incoming {
		if _, dup := seen[r.VideoID]; dup {
			continue
		}
		seen[r.VideoID] = struct{}{}
		existing = append(existing, r)
		added++
	}
	return existing, added
}

// NextFetchSize returns the total to request from yt-dlp for the next
// infinite-scroll fetch, or 0 when current has reached max.
func NextFetchSize(current, batch, max int) int {
	if current >= max {
		return 0
	}
	if n := current + batch; n < max {
		return n
	}
	return max
}

// VisibleRange returns the half-open row window [start, end) that keeps
// cursor visible (roughly centered) in a viewport of the given height.
func VisibleRange(cursor, total, height int) (int, int) {
	if height <= 0 || total <= 0 {
		return 0, 0
	}
	if total <= height {
		return 0, total
	}
	start := cursor - height/2
	if start < 0 {
		start = 0
	}
	if start+height > total {
		start = total - height
	}
	return start, start + height
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/core/ -v`
Expected: PASS (including pre-existing search/queue tests)

- [ ] **Step 5: Commit**

```bash
git add internal/core/paging.go internal/core/paging_test.go
git commit -m "feat(core): MergeResults, NextFetchSize, VisibleRange paging helpers"
```

---

### Task 8: Facade — un-truncated cache hits, `SearchMore`, `CacheSearch`

**Files:**
- Modify: `internal/tui/facade.go` (`Search` ~line 40; new methods after it)
- Test: `internal/tui/facade_test.go` (new)

**Interfaces:**
- Consumes: `ports.Searcher`, `ports.Store`
- Produces: `(f *Facade) SearchMore(ctx, query string, fetchTotal int) ([]core.SearchResult, error)`, `(f *Facade) CacheSearch(ctx, query string, results []core.SearchResult)`; changed behavior: `Search` returns the FULL cached list on cache hit (no truncation to limit). Task 9 consumes.

- [ ] **Step 1: Write the failing test** — `internal/tui/facade_test.go`. The stub store implements all of `ports.Store` as no-ops except the two search-cache methods:

```go
package tui

import (
	"context"
	"testing"
	"time"

	"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/core"
)

type fakeSearcher struct {
	lastLimit int
	results   []core.SearchResult
}

func (f *fakeSearcher) Search(ctx context.Context, query string, limit int) ([]core.SearchResult, error) {
	f.lastLimit = limit
	return f.results, nil
}
func (f *fakeSearcher) GetStreamURL(ctx context.Context, videoID string) (string, error) {
	return "http://stream/" + videoID, nil
}

type stubStore struct {
	cached      []core.SearchResult
	lastCached  []core.SearchResult
	cachedQuery string
}

func (s *stubStore) SaveHistory(ctx context.Context, e core.HistoryEntry) error { return nil }
func (s *stubStore) GetHistory(ctx context.Context, l, o int) ([]core.HistoryEntry, error) {
	return nil, nil
}
func (s *stubStore) SearchHistory(ctx context.Context, q string, l int) ([]core.HistoryEntry, error) {
	return nil, nil
}
func (s *stubStore) SaveQueue(ctx context.Context, t []core.Track, c int) error { return nil }
func (s *stubStore) LoadQueue(ctx context.Context) ([]core.Track, int, error)  { return nil, 0, nil }
func (s *stubStore) CacheSearchResults(ctx context.Context, q string, r []core.SearchResult, ttl time.Duration) error {
	s.cachedQuery, s.lastCached = q, r
	return nil
}
func (s *stubStore) GetCachedSearch(ctx context.Context, q string) ([]core.SearchResult, bool, error) {
	return s.cached, len(s.cached) > 0, nil
}
func (s *stubStore) CacheLyrics(ctx context.Context, v, l string) error { return nil }
func (s *stubStore) GetCachedLyrics(ctx context.Context, v string) (string, bool, error) {
	return "", false, nil
}
func (s *stubStore) SaveDownload(ctx context.Context, d core.Download) error { return nil }
func (s *stubStore) ListDownloads(ctx context.Context, l int) ([]core.Download, error) {
	return nil, nil
}
func (s *stubStore) CreatePlaylist(ctx context.Context, n string) (core.Playlist, error) {
	return core.Playlist{}, nil
}
func (s *stubStore) ListPlaylists(ctx context.Context) ([]core.Playlist, error) { return nil, nil }
func (s *stubStore) GetPlaylist(ctx context.Context, id int) (core.Playlist, error) {
	return core.Playlist{}, nil
}
func (s *stubStore) DeletePlaylist(ctx context.Context, id int) error { return nil }
func (s *stubStore) AddToPlaylist(ctx context.Context, id int, t core.Track) error { return nil }
func (s *stubStore) RemoveFromPlaylist(ctx context.Context, id, pos int) error     { return nil }
func (s *stubStore) Close() error                                                  { return nil }

func nResults(n int) []core.SearchResult {
	out := make([]core.SearchResult, n)
	for i := range out {
		out[i] = core.SearchResult{VideoID: string(rune('a' + i))}
	}
	return out
}

func TestSearchCacheHitNotTruncated(t *testing.T) {
	st := &stubStore{cached: nResults(30)}
	f := NewFacade(&fakeSearcher{}, nil, st)
	got, err := f.Search(context.Background(), "q", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 30 {
		t.Errorf("cache hit returned %d results, want the full 30", len(got))
	}
}

func TestSearchMoreBypassesCache(t *testing.T) {
	se := &fakeSearcher{results: nResults(5)}
	st := &stubStore{cached: nResults(30)} // cache would hit — must be ignored
	f := NewFacade(se, nil, st)
	got, err := f.SearchMore(context.Background(), "q", 35)
	if err != nil {
		t.Fatal(err)
	}
	if se.lastLimit != 35 {
		t.Errorf("searcher got limit %d, want 35", se.lastLimit)
	}
	if len(got) != 5 {
		t.Errorf("got %d results, want the searcher's 5", len(got))
	}
}

func TestCacheSearchOverwrites(t *testing.T) {
	st := &stubStore{}
	f := NewFacade(&fakeSearcher{}, nil, st)
	f.CacheSearch(context.Background(), "q", nResults(12))
	if st.cachedQuery != "q" || len(st.lastCached) != 12 {
		t.Errorf("cached %q/%d, want q/12", st.cachedQuery, len(st.lastCached))
	}
}
```

(`NewFacade(…, nil, …)` passes a nil Player — fine, these paths never touch it.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/ -v`
Expected: `TestSearchCacheHitNotTruncated` FAILS (returns 10) and compile error for `SearchMore`/`CacheSearch`.

- [ ] **Step 3: Implement** — in `internal/tui/facade.go`:

Replace the cache-hit block inside `Search`:

```go
	// Check cache first — return the full accumulated list, never truncated.
	if cached, ok, _ := f.store.GetCachedSearch(ctx, query); ok && len(cached) > 0 {
		return cached, nil
	}
```

Add after `Search`:

```go
// SearchMore fetches the first fetchTotal results for query directly from
// the searcher, bypassing the cache. Used by infinite-scroll accumulation:
// yt-dlp has no cursor, so "more" means refetching a larger prefix.
func (f *Facade) SearchMore(ctx context.Context, query string, fetchTotal int) ([]core.SearchResult, error) {
	return f.searcher.Search(ctx, query, fetchTotal)
}

// CacheSearch overwrites the cached result list for query (best-effort).
func (f *Facade) CacheSearch(ctx context.Context, query string, results []core.SearchResult) {
	_ = f.store.CacheSearchResults(ctx, query, results, time.Hour)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... `
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui/facade.go internal/tui/facade_test.go
git commit -m "feat(tui): SearchMore + CacheSearch; cache hits return full accumulated list"
```

---

### Task 9: Infinite-scroll TUI wiring + windowed search render

**Files:**
- Modify: `internal/tui/messages.go`
- Modify: `internal/tui/app.go` — struct fields, `Update` (`SearchResultMsg` ~line 569, new `SearchMoreMsg` case, `up`/`down` key cases ~line 475, wheel in `handleMouse`), `renderContent` (~line 768), `renderSearchView` (~line 790), new `doSearchMore`/`maybeLoadMore`

**Interfaces:**
- Consumes: `core.MergeResults`, `core.NextFetchSize`, `core.VisibleRange` (Task 7), `Facade.SearchMore`/`CacheSearch` (Task 8), `Config.MaxSearchResults` (Task 4)
- Produces: user-visible behavior only.

- [ ] **Step 1: Add message type** — append to `internal/tui/messages.go`:

```go
// SearchMoreMsg carries an infinite-scroll refetch. Results is the full
// refetched prefix (from index 0) — the receiver dedupes against what it has.
type SearchMoreMsg struct {
	Results []core.SearchResult
	Query   string
	Err     error
}
```

- [ ] **Step 2: Add state + constant** — in `internal/tui/app.go`, under the search-state fields (~line 72):

```go
	searchExhausted   bool // no more results available
	searchLoadingMore bool // a refetch is in flight
	searchFetchFailed bool // last refetch failed; retry on next bottom gesture
```

Near the view constants at the top of the file:

```go
// searchBatch is how many additional results each infinite-scroll fetch requests.
const searchBatch = 25
```

- [ ] **Step 3: Fetch commands** — next to `doSearch` (~line 1258):

```go
func (a *App) doSearchMore(query string, fetchTotal int) tea.Cmd {
	return func() tea.Msg {
		results, err := a.facade.SearchMore(context.Background(), query, fetchTotal)
		return SearchMoreMsg{Results: results, Query: query, Err: err}
	}
}

// maybeLoadMore triggers a background refetch when the cursor nears the end
// of the loaded results. Returns nil when no fetch is needed.
func (a *App) maybeLoadMore() tea.Cmd {
	if a.currentView != viewSearch || a.searchQuery == "" {
		return nil
	}
	if a.searchLoadingMore || a.searchExhausted {
		return nil
	}
	if a.searchCursor < len(a.searchResults)-3 {
		return nil
	}
	// After a failure, only retry from the very last row (one retry per gesture).
	if a.searchFetchFailed && a.searchCursor < len(a.searchResults)-1 {
		return nil
	}
	a.searchFetchFailed = false
	max := a.cfg.MaxSearchResults
	if max <= 0 {
		max = 100
	}
	next := core.NextFetchSize(len(a.searchResults), searchBatch, max)
	if next == 0 {
		a.searchExhausted = true
		return nil
	}
	a.searchLoadingMore = true
	return a.doSearchMore(a.searchQuery, next)
}
```

- [ ] **Step 4: Hook cursor movement** — in `Update`, replace the `down` key case (~line 480):

```go
		case "down":
			if a.prompt.Value() == "" {
				a.moveCursorDown()
				if cmd := a.maybeLoadMore(); cmd != nil {
					cmds = append(cmds, cmd)
				}
				return a, tea.Batch(cmds...)
			}
```

In `handleMouse` (Task 5), extend the wheel-down branch the same way:

```go
	if msg.Button == tea.MouseButtonWheelDown && msg.Action == tea.MouseActionPress {
		a.moveCursorDown()
		return a.maybeLoadMore()
	}
```

- [ ] **Step 5: Handle messages** — in `Update`'s message switch:

Reset accumulation state in the existing `SearchResultMsg` success branch (after `a.searchCursor = 0`):

```go
			a.searchExhausted = false
			a.searchLoadingMore = false
			a.searchFetchFailed = false
```

Add a new case after `SearchResultMsg`:

```go
	case SearchMoreMsg:
		a.searchLoadingMore = false
		if msg.Query == a.searchQuery { // ignore stale responses
			if msg.Err != nil {
				a.searchFetchFailed = true
				cmds = append(cmds, a.toast.Show("Load more failed: "+msg.Err.Error(), true))
			} else {
				merged, added := core.MergeResults(a.searchResults, msg.Results)
				a.searchResults = merged
				if added == 0 {
					a.searchExhausted = true
				} else {
					a.facade.CacheSearch(context.Background(), a.searchQuery, merged)
				}
				max := a.cfg.MaxSearchResults
				if max <= 0 {
					max = 100
				}
				if len(merged) >= max {
					a.searchExhausted = true
				}
			}
		}
```

- [ ] **Step 6: Windowed render + sentinel row** — change `renderSearchView` to take the available height. In `renderContent`, change the call:

```go
	case viewSearch:
		content = a.renderSearchView(height)
```

In `renderSearchView(height int)`, replace the full-list loop (`for i, r := range a.searchResults`) with a window:

```go
	// Window the list so the cursor stays visible: header block above uses
	// 2 rows, the footer hint 2, the sentinel 1.
	listRows := height - 5
	if listRows < 3 {
		listRows = 3
	}
	start, end := core.VisibleRange(a.searchCursor, len(a.searchResults), listRows)
	for i := start; i < end; i++ {
		r := a.searchResults[i]
		cursor := "  "
		if i == a.searchCursor {
			cursor = a.styles.Accent.Render("> ")
		}
		num := a.styles.Muted.Render(fmt.Sprintf("%2d. ", i+1))
		dur := formatDuration(r.Duration)
		title := r.Title
		if i == a.searchCursor {
			title = a.styles.Selected.Render(title)
		}
		channel := a.styles.Muted.Render(" - " + r.Channel)
		duration := a.styles.Muted.Render(" [" + dur + "]")
		b.WriteString(cursor + num + title + channel + duration + "\n")
	}

	// Sentinel row: fetch status at the bottom of the loaded list.
	switch {
	case a.searchLoadingMore:
		b.WriteString(a.styles.Muted.Render("     … loading more") + "\n")
	case a.searchFetchFailed:
		b.WriteString(a.styles.Warning.Render("     fetch failed — scroll to retry") + "\n")
	case a.searchExhausted && end == len(a.searchResults):
		b.WriteString(a.styles.Muted.Render("     — end of results —") + "\n")
	}
```

(Everything above the loop — welcome screen, loading, header — stays. `a.styles.Warning` exists in `internal/tui/theme/theme.go`.)

- [ ] **Step 7: Build + test + manual check**

Run: `go build ./... && go test ./...`
Expected: PASS.
Manual: search something popular; hold `down` — at row ~8 the `… loading more` sentinel appears, list grows past 10, keeps growing to 100, then `— end of results —`; row numbers continue (11., 12., …); cursor never leaves the visible window; re-running the same search immediately restores the long list (cache).

- [ ] **Step 8: Commit**

```bash
git add internal/tui/messages.go internal/tui/app.go
git commit -m "feat(tui): infinite-scroll search with windowed rendering and fetch sentinels"
```

---

### Task 10: `window` subcommand — terminal resolver + launcher

**Files:**
- Create: `internal/window/window.go`
- Test: `internal/window/window_test.go`
- Modify: `internal/config/config.go` (add `[window]` section)
- Modify: `cmd/wrkmon-go/main.go` (dispatch before adapter startup)

**Interfaces:**
- Consumes: `config.Load()`
- Produces: `window.Resolve(self, override string, extra []string, lookPath func(string) (string, error)) (bin string, args []string, err error)`, `window.Launch(override string, extra []string) error`, `config.WindowConfig{Terminal string; ExtraArgs []string}` as `Config.Window`.

- [ ] **Step 1: Write the failing test** — `internal/window/window_test.go`:

```go
package window

import (
	"runtime"
	"strings"
	"testing"
)

func fakeLook(installed ...string) func(string) (string, error) {
	set := map[string]bool{}
	for _, b := range installed {
		set[b] = true
	}
	return func(name string) (string, error) {
		if set[name] {
			return "/usr/bin/" + name, nil
		}
		return "", &notFoundError{name}
	}
}

type notFoundError struct{ name string }

func (e *notFoundError) Error() string { return e.name + " not found" }

func TestResolveAutoPicksFirstInstalled(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("linux/darwin table")
	}
	bin, args, err := Resolve("/opt/wrkmon-go", "auto", nil, fakeLook("alacritty", "xterm"))
	if err != nil {
		t.Fatal(err)
	}
	if bin != "/usr/bin/alacritty" {
		t.Errorf("bin = %s, want alacritty (first installed in priority order)", bin)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "wrkmon-go") || !strings.Contains(joined, "/opt/wrkmon-go") {
		t.Errorf("args missing class or self path: %v", args)
	}
}

func TestResolveKittyArgv(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("linux/darwin table")
	}
	_, args, err := Resolve("/opt/wrkmon-go", "auto", nil, fakeLook("kitty"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"--class=wrkmon-go", "--title=wrkmon", "-e", "/opt/wrkmon-go"}
	if len(args) != len(want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Errorf("args[%d] = %s, want %s", i, args[i], want[i])
		}
	}
}

func TestResolveOverrideHonored(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("linux/darwin table")
	}
	bin, _, err := Resolve("/opt/wrkmon-go", "foot", nil, fakeLook("kitty", "foot"))
	if err != nil {
		t.Fatal(err)
	}
	if bin != "/usr/bin/foot" {
		t.Errorf("bin = %s, want foot (override)", bin)
	}
}

func TestResolveUnknownOverride(t *testing.T) {
	_, _, err := Resolve("/opt/wrkmon-go", "hyperterm", nil, fakeLook("kitty"))
	if err == nil {
		t.Error("expected error for unknown terminal name")
	}
}

func TestResolveNoneInstalled(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("linux/darwin table")
	}
	_, _, err := Resolve("/opt/wrkmon-go", "auto", nil, fakeLook())
	if err == nil {
		t.Fatal("expected error when nothing installed")
	}
	if !strings.Contains(err.Error(), "kitty") {
		t.Errorf("error should list supported terminals, got: %v", err)
	}
}

func TestResolveExtraArgs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("linux/darwin table")
	}
	_, args, err := Resolve("/opt/wrkmon-go", "kitty", []string{"--font-size=14"}, fakeLook("kitty"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(args, " "), "--font-size=14") {
		t.Errorf("extra args not passed through: %v", args)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/window/ -v`
Expected: compile error — package doesn't exist yet

- [ ] **Step 3: Implement** — `internal/window/window.go`:

```go
// Package window launches wrkmon-go inside a dedicated terminal-emulator
// window in "app mode" (own window class, no tabs), so the TUI behaves
// like a desktop application. The window class is always "wrkmon-go" and
// must match StartupWMClass in assets/wrkmon-go.desktop.
package window

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// termSpec describes one supported terminal emulator.
type termSpec struct {
	name string
	args func(self string, extra []string) []string
}

// specs returns the priority-ordered launch table for this OS.
func specs() []termSpec {
	switch runtime.GOOS {
	case "windows":
		return []termSpec{
			{"wt", func(self string, extra []string) []string {
				return append(append([]string{"--title", "wrkmon"}, extra...), self)
			}},
		}
	default: // linux, darwin, bsd — X11/Wayland terminals
		return []termSpec{
			{"kitty", func(self string, extra []string) []string {
				return append(append([]string{"--class=wrkmon-go", "--title=wrkmon"}, extra...), "-e", self)
			}},
			{"alacritty", func(self string, extra []string) []string {
				return append(append([]string{"--class", "wrkmon-go", "--title", "wrkmon"}, extra...), "-e", self)
			}},
			{"wezterm", func(self string, extra []string) []string {
				return append(append([]string{"start", "--class", "wrkmon-go"}, extra...), "--", self)
			}},
			{"ghostty", func(self string, extra []string) []string {
				return append(append([]string{"--class=wrkmon-go"}, extra...), "-e", self)
			}},
			{"foot", func(self string, extra []string) []string {
				return append(append([]string{"--app-id=wrkmon-go", "--title=wrkmon"}, extra...), self)
			}},
			{"cosmic-term", func(self string, extra []string) []string {
				return append(append([]string{}, extra...), "--", self)
			}},
			{"gnome-terminal", func(self string, extra []string) []string {
				return append(append([]string{"--title=wrkmon"}, extra...), "--", self)
			}},
			{"konsole", func(self string, extra []string) []string {
				return append(append([]string{"--separate"}, extra...), "-e", self)
			}},
			{"xterm", func(self string, extra []string) []string {
				return append(append([]string{"-class", "wrkmon-go", "-T", "wrkmon"}, extra...), "-e", self)
			}},
		}
	}
}

// Resolve picks the terminal binary and argv. override is "auto" (or "")
// to probe the table in order, or a terminal name from the table.
// lookPath is exec.LookPath, injectable for tests.
func Resolve(self, override string, extra []string, lookPath func(string) (string, error)) (string, []string, error) {
	table := specs()

	if override != "" && override != "auto" {
		for _, s := range table {
			if s.name == override {
				bin, err := lookPath(s.name)
				if err != nil {
					return "", nil, fmt.Errorf("configured terminal %q is not installed", override)
				}
				return bin, s.args(self, extra), nil
			}
		}
		return "", nil, fmt.Errorf("unknown terminal %q; supported: %s", override, names(table))
	}

	for _, s := range table {
		if bin, err := lookPath(s.name); err == nil {
			return bin, s.args(self, extra), nil
		}
	}
	return "", nil, fmt.Errorf(
		"no supported terminal found (looked for %s); set [window] terminal in ~/.config/wrkmon-go/config.toml",
		names(table))
}

func names(table []termSpec) string {
	out := make([]string, len(table))
	for i, s := range table {
		out[i] = s.name
	}
	return strings.Join(out, ", ")
}

// Launch opens the app window and returns once the terminal is spawned.
func Launch(override string, extra []string) error {
	self, err := os.Executable()
	if err != nil {
		return err
	}
	bin, args, err := Resolve(self, override, extra, exec.LookPath)
	if err != nil {
		// OS-level fallbacks that aren't PATH-probed terminals.
		switch runtime.GOOS {
		case "windows":
			return exec.Command("cmd", "/c", "start", "wrkmon", self).Start()
		case "darwin":
			return exec.Command("osascript", "-e",
				fmt.Sprintf(`tell application "Terminal" to do script %q`, self)).Start()
		}
		return err
	}
	cmd := exec.Command(bin, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("launching %s: %w", bin, err)
	}
	return cmd.Process.Release()
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/window/ -v`
Expected: PASS

- [ ] **Step 5: Config `[window]` section** — in `internal/config/config.go`:

```go
// WindowConfig controls the `wrkmon-go window` launcher.
type WindowConfig struct {
	Terminal  string   `toml:"terminal"`   // "auto" or a supported terminal name
	ExtraArgs []string `toml:"extra_args"` // appended to the terminal's argv
}
```

Add to `Config`: `Window WindowConfig \`toml:"window"\`` and to `DefaultConfig()`: `Window: WindowConfig{Terminal: "auto"},`

Append to `internal/config/config_test.go`:

```go
func TestWindowDefaults(t *testing.T) {
	setTempHome(t)
	cfg := Load()
	if cfg.Window.Terminal != "auto" {
		t.Errorf("Window.Terminal = %q, want auto", cfg.Window.Terminal)
	}
}
```

- [ ] **Step 6: Dispatch in main** — in `cmd/wrkmon-go/main.go`, at the very top of `main()` (before adapter startup, so `window` works even when mpv/yt-dlp are missing):

```go
	if len(os.Args) > 1 && (os.Args[1] == "window" || os.Args[1] == "--window") {
		cfg := config.Load()
		if err := window.Launch(cfg.Window.Terminal, cfg.Window.ExtraArgs); err != nil {
			fmt.Fprintln(os.Stderr, "wrkmon-go window:", err)
			os.Exit(1)
		}
		return
	}
```

Add the import: `"github.com/Umar-Khan-Yousafzai/wrkmon-go/internal/window"`.

- [ ] **Step 7: Build + test + manual check**

Run: `go build ./... && go test ./...`
Expected: PASS.
Manual: `go build -o /tmp/wrkmon-go ./cmd/wrkmon-go && /tmp/wrkmon-go window` → a new terminal window opens running the TUI, launcher exits immediately; `[window] terminal = "kitty"` override respected; bogus override prints the supported list.

- [ ] **Step 8: Commit**

```bash
git add internal/window/ internal/config/ cmd/wrkmon-go/main.go
git commit -m "feat(window): 'wrkmon-go window' launches the TUI in its own terminal window"
```

---

### Task 11: Desktop assets + installer/packaging integration

**Files:**
- Create: `assets/wrkmon-go.desktop`, `assets/icon.png` (generated), `scripts/gen-icon.go`
- Modify: `scripts/install.sh`, `scripts/uninstall.sh`, `.github/workflows/release.yml` (deb step, ~line 64), `scripts/installer-gui.ps1` (~line 304)

**Interfaces:**
- Consumes: `wrkmon-go window` (Task 10); WM class `wrkmon-go`
- Produces: desktop entry + icon shipped by curl installer and .deb; Windows Start-Menu shortcut passes the `window` argument.

- [ ] **Step 1: Desktop entry** — `assets/wrkmon-go.desktop`:

```ini
[Desktop Entry]
Type=Application
Name=wrkmon
GenericName=YouTube Audio Player
Comment=TUI YouTube audio player
Exec=wrkmon-go window
Icon=wrkmon-go
Terminal=false
Categories=AudioVideo;Audio;Player;
Keywords=youtube;music;audio;tui;
StartupWMClass=wrkmon-go
```

- [ ] **Step 2: Icon generator** — `scripts/gen-icon.go` (stdlib-only, run once, commit the PNG):

```go
//go:build ignore

// Generates assets/icon.png: 256×256, dark rounded square with an accent
// play triangle. Run: go run scripts/gen-icon.go
package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
)

func main() {
	const size = 256
	bg := color.NRGBA{0x12, 0x12, 0x14, 0xff}     // near-black
	accent := color.NRGBA{0xfa, 0xfa, 0xfa, 0xff} // opencode-mono white
	img := image.NewNRGBA(image.Rect(0, 0, size, size))

	corner := 40.0
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			// Rounded-rect mask
			dx, dy := 0.0, 0.0
			if float64(x) < corner {
				dx = corner - float64(x)
			} else if float64(x) > size-corner {
				dx = float64(x) - (size - corner)
			}
			if float64(y) < corner {
				dy = corner - float64(y)
			} else if float64(y) > size-corner {
				dy = float64(y) - (size - corner)
			}
			if dx*dx+dy*dy > corner*corner {
				continue // transparent corner
			}
			img.Set(x, y, bg)
		}
	}

	// Play triangle: vertices (96,72), (96,184), (184,128)
	for y := 72; y <= 184; y++ {
		half := 1.0 - abs(float64(y)-128)/56.0 // 1 at center, 0 at tips
		right := 96 + int(half*88)
		for x := 96; x <= right; x++ {
			img.Set(x, y, accent)
		}
	}

	f, err := os.Create("assets/icon.png")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		log.Fatal(err)
	}
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
```

Run: `go run scripts/gen-icon.go` then verify `assets/icon.png` exists and opens as a 256×256 PNG (`file assets/icon.png` → `PNG image data, 256 x 256`).

- [ ] **Step 3: install.sh desktop integration** — in `scripts/install.sh`, after the binary-install step (at the end of the Linux flow, before the final success message), add:

```bash
# Desktop entry + icon (Linux only)
if [ "$GOOS" = "linux" ]; then
    info "Installing desktop entry..."
    APP_DIR="${HOME}/.local/share/applications"
    ICON_DIR="${HOME}/.local/share/icons/hicolor/256x256/apps"
    mkdir -p "$APP_DIR" "$ICON_DIR"
    RAW="https://raw.githubusercontent.com/${REPO}/main"
    if curl -fsSL "${RAW}/assets/wrkmon-go.desktop" -o "${APP_DIR}/wrkmon-go.desktop" \
       && curl -fsSL "${RAW}/assets/icon.png" -o "${ICON_DIR}/wrkmon-go.png"; then
        command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database "$APP_DIR" || true
        ok "Desktop entry installed — find 'wrkmon' in your app launcher"
    else
        warn "Desktop entry download failed (app still works from the terminal)"
    fi
fi
```

Then, in the same spot, add the macOS equivalent — a minimal `.app` bundle so "wrkmon" appears in Launchpad/Spotlight:

```bash
# App bundle (macOS only)
if [ "$GOOS" = "darwin" ]; then
    info "Installing app bundle..."
    APP="${HOME}/Applications/wrkmon.app"
    mkdir -p "${APP}/Contents/MacOS"
    cat > "${APP}/Contents/Info.plist" <<'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
    <key>CFBundleName</key><string>wrkmon</string>
    <key>CFBundleIdentifier</key><string>com.umarkhan.wrkmon-go</string>
    <key>CFBundleExecutable</key><string>wrkmon-launcher</string>
    <key>CFBundlePackageType</key><string>APPL</string>
</dict></plist>
EOF
    cat > "${APP}/Contents/MacOS/wrkmon-launcher" <<EOF
#!/bin/bash
exec "${INSTALL_DIR}/${BINARY}" window
EOF
    chmod +x "${APP}/Contents/MacOS/wrkmon-launcher"
    ok "App bundle installed at ~/Applications/wrkmon.app"
fi
```

And in `scripts/uninstall.sh`, alongside binary removal (macOS: also `rm -rf "${HOME}/Applications/wrkmon.app"`):

```bash
rm -f "${HOME}/.local/share/applications/wrkmon-go.desktop" \
      "${HOME}/.local/share/icons/hicolor/256x256/apps/wrkmon-go.png"
command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database "${HOME}/.local/share/applications" || true
```

- [ ] **Step 4: .deb payload** — in `.github/workflows/release.yml` "Build .deb packages" step, after the `chmod 755` line (line ~71), add inside the `for ARCH` loop:

```yaml
            mkdir -p "${PKG}/usr/share/applications" "${PKG}/usr/share/icons/hicolor/256x256/apps"
            cp assets/wrkmon-go.desktop "${PKG}/usr/share/applications/wrkmon-go.desktop"
            cp assets/icon.png "${PKG}/usr/share/icons/hicolor/256x256/apps/wrkmon-go.png"
```

(The release job checks out the repo, so `assets/` is present.)

- [ ] **Step 5: Windows Start-Menu shortcut argument** — in `scripts/installer-gui.ps1`, after line 304 (`$sc.TargetPath = Join-Path $InstallDir $Binary`), add:

```powershell
            $sc.Arguments = "window"
```

(The shortcut now opens the app in Windows Terminal via the `window` subcommand; if `wt` is absent it falls back to a plain console window.)

- [ ] **Step 6: Verify + commit**

Run: `bash -n scripts/install.sh && bash -n scripts/uninstall.sh && go vet ./... && go test ./...`
Expected: no syntax errors, tests PASS.
Manual: copy `assets/wrkmon-go.desktop` to `~/.local/share/applications/` and the icon to `~/.local/share/icons/hicolor/256x256/apps/wrkmon-go.png`, then launch "wrkmon" from the app launcher → own window with the icon, dock groups under one entry (WM class `wrkmon-go`).

```bash
git add assets/ scripts/gen-icon.go scripts/install.sh scripts/uninstall.sh .github/workflows/release.yml scripts/installer-gui.ps1
git commit -m "feat(packaging): desktop entry, icon, deb payload, Start-Menu window shortcut"
```

---

### Task 12: Final verification sweep

**Files:** none (verification only)

- [ ] **Step 1: Full test + vet + fmt**

Run: `gofmt -l . && go vet ./... && go test ./...`
Expected: gofmt prints nothing; vet clean; all tests PASS.

- [ ] **Step 2: Manual checklist** (run the app for each):

1. Play a track → one-time seek-hint toast.
2. `→` ±5s with toast; `shift+→` ±30s while the prompt contains text.
3. `/seek 1:23`, `/seek 50%`, `/seek +30`, `/seek garbage` (usage toast), `/seek 10` with nothing playing ("nothing playing").
4. Click and drag the statusbar bar; click the big bar in `/now`.
5. `mouse = false` in config → no mouse capture, text selection works.
6. Search → hold `down` / wheel: sentinel `… loading more`, list grows past 10 → cap 100 → `— end of results —`; repeat search restores instantly from cache.
7. Disconnect network mid-scroll → `fetch failed — scroll to retry`, reconnect, scroll retries.
8. `wrkmon-go window` opens a dedicated window; `.desktop` launch groups in dock; config override works; bogus override lists supported terminals.

- [ ] **Step 3: Push**

```bash
git push origin main
```
