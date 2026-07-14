# Media Keys + EQ + Focus Mode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Hardware media-key control (Linux MPRIS full / Windows hotkeys / macOS partial), an 18-band equalizer with presets + custom bands, and the v1-style fake-work-screen focus mode — plus a `--version` flag.

**Architecture:** New `core.MediaRemote` port with per-GOOS adapters under `internal/adapters/mediakeys/`; EQ as pure state in `core/eq.go` applied through a new `MPV.SetAudioFilter`; focus mode as pure text generators in `internal/tui/focus/` rendered by an overlay flag in the App model. All commands flow through the existing `commands.Dispatcher` sentinel-string pattern.

**Tech Stack:** Go 1.25, Bubble Tea, godbus/dbus/v5 (new), golang.org/x/sys (new, windows-only), mpv JSON IPC, FFmpeg superequalizer.

**Spec:** `docs/superpowers/specs/2026-07-14-mediakeys-eq-focus-design.md` — read it first.

## Global Constraints

- Zero CGO. Any dependency must be pure Go.
- `gofmt`-clean; `go vet ./...` clean.
- Config keys are FLAT in `config.toml` (match existing `Config` struct in `internal/config/config.go:17`).
- Commit messages: conventional commits, NO AI/Co-Authored-By attribution lines.
- `go get` needs network → run those commands with sandbox off.
- Never block startup: every adapter failure degrades to a no-op with a logged/flash warning.
- Slash-command handlers return `(string, error)`; side-effects beyond the Facade go through sentinel strings handled in `handleSubmit` (`internal/tui/app.go:1202`) — follow the existing `"LOAD_DOWNLOADS"` / `"DOWNLOAD:..."` pattern.
- Tests: table-driven where natural; anything needing a real bus/mpv goes behind `testing.Short()` guard.

---

### Task 1: `--version` flag

**Files:**
- Modify: `cmd/wrkmon-go/main.go` (top of `main`, before the `window` check)
- Test: none (trivial CLI branch; verified by command)

**Interfaces:**
- Consumes: existing `var version = "dev"` (main.go:18)
- Produces: `wrkmon-go --version` | `-v` | `version` → stdout `wrkmon-go <version>`, exit 0.

- [ ] **Step 1: Implement**

```go
// first lines of main(), before the window subcommand check
if len(os.Args) > 1 {
    switch os.Args[1] {
    case "--version", "-v", "version":
        fmt.Printf("wrkmon-go %s\n", version)
        return
    }
}
```

- [ ] **Step 2: Verify**

Run: `go build -ldflags "-X main.version=test-stamp" -o /tmp/wg ./cmd/wrkmon-go && /tmp/wg --version`
Expected: `wrkmon-go test-stamp`

- [ ] **Step 3: Commit** — `feat(cli): add --version flag`

---

### Task 2: EQ core model

**Files:**
- Create: `core/eq.go`
- Test: `core/eq_test.go`

**Interfaces:**
- Produces (later tasks depend on these EXACT names):
  - `type EQState struct { Preset string; Gains [18]float64; Enabled bool }` (gains in dB, −12..+12)
  - `func EQPreset(name string) (EQState, bool)` — names: `flat`, `bass`, `treble`, `vocal`, `rock`, `pop`
  - `func (e EQState) FilterString() string` — mpv lavfi string, `""` when disabled or all-zero
  - `func (e *EQState) SetBand(n int, db float64) error` — n is 1-based [1,18], db [−12,+12]; sets Preset="custom"
  - `var EQPresetNames = []string{...}` (sorted, for /eq feedback + tests)

- [ ] **Step 1: Write failing tests** (`core/eq_test.go`)

```go
func TestEQPresetKnown(t *testing.T) {
    e, ok := core.EQPreset("bass")
    if !ok || e.Preset != "bass" || !e.Enabled { t.Fatalf("bass preset broken: %+v ok=%v", e, ok) }
    if e.Gains[0] <= 0 { t.Fatalf("bass must boost low bands, got %v", e.Gains[0]) }
}
func TestEQPresetUnknown(t *testing.T) {
    if _, ok := core.EQPreset("nope"); ok { t.Fatal("unknown preset must return ok=false") }
}
func TestSetBandValidation(t *testing.T) {
    var e core.EQState
    for _, c := range []struct{ n int; db float64; wantErr bool }{
        {0, 3, true}, {19, 3, true}, {1, 12.1, true}, {1, -12.1, true}, {1, -12, false}, {18, 12, false},
    } {
        if err := e.SetBand(c.n, c.db); (err != nil) != c.wantErr {
            t.Errorf("SetBand(%d,%v) err=%v want err=%v", c.n, c.db, err, c.wantErr)
        }
    }
    if e.Preset != "custom" { t.Fatalf("valid SetBand must set preset=custom, got %q", e.Preset) }
}
func TestFilterString(t *testing.T) {
    var e core.EQState
    if got := e.FilterString(); got != "" { t.Fatalf("disabled/flat => empty, got %q", got) }
    e.Enabled = true
    if got := e.FilterString(); got != "" { t.Fatalf("all-zero gains => empty, got %q", got) }
    e.Gains[0] = 6 // +6 dB => multiplier 10^(6/20) ≈ 1.995
    got := e.FilterString()
    if !strings.HasPrefix(got, "lavfi=[superequalizer=") || !strings.Contains(got, "1b=2.0") {
        t.Fatalf("bad filter string: %q", got)
    }
}
```

- [ ] **Step 2: Run** `go test ./core/ -run TestEQ -v` → FAIL (undefined symbols)

- [ ] **Step 3: Implement** (`core/eq.go`)

```go
package core

import (
    "fmt"
    "math"
    "strings"
)

// EQState holds the 18-band superequalizer state. Gains are in dB (−12..+12).
type EQState struct {
    Preset  string
    Gains   [18]float64
    Enabled bool
}

var eqPresets = map[string][18]float64{
    "flat":   {},
    "bass":   {6, 5, 4, 3, 2, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
    "treble": {0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 2, 3, 4, 5, 5, 6},
    "vocal":  {-2, -1, 0, 1, 2, 3, 4, 4, 4, 3, 2, 1, 0, 0, -1, -1, -2, -2},
    "rock":   {5, 4, 3, 1, 0, -1, -1, 0, 1, 2, 3, 3, 4, 4, 4, 4, 4, 5},
    "pop":    {-1, 0, 1, 2, 4, 4, 3, 2, 1, 0, 0, 1, 2, 3, 3, 2, 1, 0},
}

var EQPresetNames = []string{"bass", "flat", "pop", "rock", "treble", "vocal"}

func EQPreset(name string) (EQState, bool) {
    g, ok := eqPresets[name]
    if !ok {
        return EQState{}, false
    }
    return EQState{Preset: name, Gains: g, Enabled: true}, true
}

func (e *EQState) SetBand(n int, db float64) error {
    if n < 1 || n > 18 {
        return fmt.Errorf("band must be 1-18, got %d", n)
    }
    if db < -12 || db > 12 {
        return fmt.Errorf("gain must be -12..+12 dB, got %v", db)
    }
    e.Gains[n-1] = db
    e.Preset = "custom"
    e.Enabled = true
    return nil
}

// FilterString renders the mpv af value. Empty string means "no filter".
func (e EQState) FilterString() string {
    if !e.Enabled {
        return ""
    }
    allZero := true
    for _, g := range e.Gains {
        if g != 0 {
            allZero = false
            break
        }
    }
    if allZero {
        return ""
    }
    parts := make([]string, 18)
    for i, db := range e.Gains {
        mult := math.Pow(10, db/20) // dB → linear multiplier; superequalizer range 0..20, 1=unity
        parts[i] = fmt.Sprintf("%db=%.1f", i+1, mult)
    }
    return "lavfi=[superequalizer=" + strings.Join(parts, ":") + "]"
}
```

- [ ] **Step 4: Run** `go test ./core/ -run "TestEQ|TestSetBand|TestFilterString" -v` → PASS
- [ ] **Step 5: Commit** — `feat(core): 18-band EQ state with presets and custom bands`

---

### Task 3: mpv SetAudioFilter + live probe

**Files:**
- Modify: `internal/adapters/mpv/mpv.go` (new method next to `SetVolume`, mpv.go:183)
- Modify: `internal/tui/facade.go` (passthrough next to `SetVolume`, facade.go:182)
- Test: `internal/adapters/mpv/eqprobe_test.go` (integration, Short-guarded)

**Interfaces:**
- Produces: `func (m *MPV) SetAudioFilter(filter string) error`; `func (f *Facade) SetAudioFilter(filter string) error`. Empty string clears the chain.

- [ ] **Step 1: Implement adapter method**

```go
// SetAudioFilter sets (or clears, with "") the mpv audio filter chain.
func (m *MPV) SetAudioFilter(filter string) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    _, err := m.command("set_property", "af", filter)
    return err
}
```

(Match the lock discipline of the neighboring methods — read `SetVolume`/`Pause` first; if they don't lock, don't lock.)

- [ ] **Step 2: Facade passthrough**

```go
func (f *Facade) SetAudioFilter(filter string) error {
    return f.player.SetAudioFilter(filter)
}
```

If `f.player` is a narrower interface than `*mpv.MPV`, add `SetAudioFilter(string) error` to that interface and to any test fake implementing it.

- [ ] **Step 3: Probe test** (`internal/adapters/mpv/eqprobe_test.go`) — verifies the superequalizer syntax against REAL mpv 0.37:

```go
func TestSetAudioFilter_Integration(t *testing.T) {
    if testing.Short() { t.Skip("integration") }
    if _, err := exec.LookPath("mpv"); err != nil { t.Skip("mpv not installed") }
    m, err := New("")
    if err != nil { t.Fatal(err) }
    // Play silence so an af chain can attach: av:lavfi generates a tone without network.
    if err := m.Play("av://lavfi:anullsrc=d=30"); err != nil { t.Fatal(err) }
    defer m.Close()
    time.Sleep(1 * time.Second)
    var e core.EQState
    e.Enabled = true
    e.Gains[0] = 6
    if err := m.SetAudioFilter(e.FilterString()); err != nil {
        t.Fatalf("mpv rejected filter %q: %v", e.FilterString(), err)
    }
    if err := m.SetAudioFilter(""); err != nil {
        t.Fatalf("clearing af failed: %v", err)
    }
}
```

If mpv rejects `lavfi=[superequalizer=...]`, adjust `FilterString` (Task 2) to the syntax mpv 0.37 accepts (`@wrkmon:lavfi=[...]` labelled form or `superequalizer=...` bare) — the probe test is the arbiter; update the Task 2 unit test prefix assertion to match, and note the final syntax in the commit body.

- [ ] **Step 4: Run** `go test ./internal/adapters/mpv/ -run TestSetAudioFilter -v` → PASS (not -short)
- [ ] **Step 5: Commit** — `feat(mpv): SetAudioFilter with live superequalizer probe`

---

### Task 4: /eq commands + persistence

**Files:**
- Modify: `internal/tui/app.go` — replace the `/eq` entry in the hidden-namespace loop (app.go:423-434) with a real registration in `buildCommands`; startup apply in `Init` (app.go:440); reapply on respawn/track start (`doPlayTrack` app.go:1550, `doPlayFromQueue`, `doNextTrack`, `doPrevTrack` — find the single choke point where playback actually starts; prefer adding the reapply inside `Facade.PlayTrack` after successful load, ONE place).
- Modify: `internal/config/config.go` — new fields + defaults.
- Test: `internal/tui/eq_cmd_test.go` (dispatcher-level), `internal/config/config_test.go` (round-trip additions).

**Interfaces:**
- Consumes: `core.EQPreset`, `EQState.SetBand`, `EQState.FilterString`, `Facade.SetAudioFilter` (Tasks 2-3).
- Produces: config fields `EQPreset string \`toml:"eq_preset"\``, `EQGains []float64 \`toml:"eq_gains"\``, `EQEnabled bool \`toml:"eq_enabled"\``; App field `eq core.EQState`.

- [ ] **Step 1: Config fields.** Add to `Config` struct; defaults: `EQPreset: "flat"`, `EQGains: nil`, `EQEnabled: false`. Round-trip test: set gains `[18]` values, Save, Load, compare. (Slice in TOML: fine. On Load, if `len(EQGains) != 18`, treat as flat — write a helper `func (c Config) EQState() core.EQState`.)

- [ ] **Step 2: Failing dispatcher tests** (`internal/tui/eq_cmd_test.go`): build the App's dispatcher with a fake facade recording `SetAudioFilter` calls; assert:
  - `/eq bass` → recorded filter contains `superequalizer`, feedback mentions `bass`.
  - `/eq off` → recorded filter `""`, feedback mentions off.
  - `/eq band 3 4.5` → filter recorded, feedback mentions band 3.
  - `/eq band 25 1` → error.
  - `/eq nope` → error listing valid presets.
  - `/eq` / `/eq show` → feedback contains preset name.

- [ ] **Step 3: Implement handler** (replace hidden stub; keep `/focus` in the hidden loop for now):

```go
d.Register(commands.Command{
    Name:        "/eq",
    Description: "Equalizer: /eq <preset>|off|show|band <1-18> <dB>",
    Handler: func(args string) (string, error) {
        fields := strings.Fields(args)
        switch {
        case len(fields) == 0 || fields[0] == "show":
            return a.eqStatusLine(), nil
        case fields[0] == "off":
            a.eq.Enabled = false
            if err := a.facade.SetAudioFilter(""); err != nil { return "", err }
            a.saveEQ()
            return "EQ off", nil
        case fields[0] == "band" && len(fields) == 3:
            n, err1 := strconv.Atoi(fields[1])
            db, err2 := strconv.ParseFloat(fields[2], 64)
            if err1 != nil || err2 != nil { return "", fmt.Errorf("usage: /eq band <1-18> <-12..12>") }
            if err := a.eq.SetBand(n, db); err != nil { return "", err }
            if err := a.facade.SetAudioFilter(a.eq.FilterString()); err != nil { return "", err }
            a.saveEQ()
            return fmt.Sprintf("EQ band %d → %+.1f dB (custom)", n, db), nil
        default:
            e, ok := core.EQPreset(fields[0])
            if !ok { return "", fmt.Errorf("unknown preset %q (have: %s, off, band)", fields[0], strings.Join(core.EQPresetNames, ", ")) }
            a.eq = e
            if err := a.facade.SetAudioFilter(a.eq.FilterString()); err != nil { return "", err }
            a.saveEQ()
            return "EQ preset: " + fields[0], nil
        }
    },
})
```

`eqStatusLine()`: preset + comma list of non-zero bands (`3:+4.5dB`), or "EQ off". `saveEQ()`: copy state into `a.cfg` fields + `a.cfg.Save()` (match how `/vol` persists — find and mirror that call pattern exactly).

- [ ] **Step 4: Startup + reapply.** In `Init()` after volume: if `cfg.EQEnabled`, set `a.eq = cfg.EQState()`. In the single playback-start choke point, after successful load: `if a.eq.Enabled { _ = a.facade.SetAudioFilter(a.eq.FilterString()) }` — also after `Respawn()`. Write one test with the fake facade proving filter reapplied on track start.

- [ ] **Step 5: Run** `go test ./internal/tui/ ./internal/config/ -v -short` → PASS. **Commit** — `feat(eq): /eq presets, custom bands, persistence, reapply on load`

---

### Task 5: Focus generators

**Files:**
- Create: `internal/tui/focus/focus.go`, `internal/tui/focus/htop.go`, `internal/tui/focus/buildlog.go`, `internal/tui/focus/testrunner.go`
- Test: `internal/tui/focus/focus_test.go`

**Interfaces:**
- Produces:
  - `type Kind int` — `KindHtop`, `KindBuildLog`, `KindTestRunner`; `func RandomKind(r *rand.Rand) Kind`
  - `func Render(kind Kind, w, h int, r *rand.Rand, tick int) string` — full-screen text, EXACTLY ≤h lines each ≤w cells; `tick` advances animation (htop re-rolls numbers; buildlog/testrunner reveal `tick`-many lines then idle).
- Port content from v1 `wrkmon/ui/screens/focus.py` (fetch: `gh api repos/Umar-Khan-Yousafzai/Wrkmon-TUI-Youtube/contents/wrkmon/ui/screens/focus.py --jq .content | base64 -d`) and fake process names from v1 `wrkmon/utils/stealth.py` (same fetch pattern): node-inspector, webpack-dev-srv, vite-hmr-watch, eslint-daemon, tsc-watch, pytest-runner, cargo-watch, go-build-srv, rust-analyzer, prettier-fmt.

- [ ] **Step 1: Failing tests**

```go
func TestRenderBounds(t *testing.T) {
    r := rand.New(rand.NewSource(42))
    for _, k := range []focus.Kind{focus.KindHtop, focus.KindBuildLog, focus.KindTestRunner} {
        for _, tick := range []int{0, 3, 50} {
            out := focus.Render(k, 100, 30, rand.New(rand.NewSource(42)), tick)
            lines := strings.Split(out, "\n")
            if len(lines) > 30 { t.Errorf("kind %v tick %d: %d lines > 30", k, tick, len(lines)) }
            for i, ln := range lines {
                if utf8.RuneCountInString(ln) > 100 { t.Errorf("kind %v line %d too wide", k, i) }
            }
        }
    }
    _ = r
}
func TestHtopLooksLikeHtop(t *testing.T) {
    out := focus.Render(focus.KindHtop, 120, 40, rand.New(rand.NewSource(1)), 0)
    for _, want := range []string{"CPU", "Mem", "Load average", "PID"} {
        if !strings.Contains(out, want) { t.Errorf("htop output missing %q", want) }
    }
    if !strings.Contains(out, "webpack-dev-srv") && !strings.Contains(out, "rust-analyzer") &&
        !strings.Contains(out, "go-build-srv") {
        t.Error("htop process table must use fake dev-tool names")
    }
}
func TestDeterministicPerSeed(t *testing.T) {
    a := focus.Render(focus.KindBuildLog, 80, 24, rand.New(rand.NewSource(7)), 5)
    b := focus.Render(focus.KindBuildLog, 80, 24, rand.New(rand.NewSource(7)), 5)
    if a != b { t.Error("same seed+tick must render identically") }
}
```

- [ ] **Step 2: Run** → FAIL. **Step 3: Implement.** Htop: header (uptime, tasks, load avg), per-CPU bar rows (`[||||||    47.3%]`, cpu count from {4,8,12,16}), Mem/Swp bars, then a PID/USER/CPU%/MEM%/TIME/Command table of 10-15 rows using the fake names; numbers derived from `r` re-seeded with `tick` mixed in so ticks change values. BuildLog: webpack/cargo-style lines (`[built] modules …`, `Compiling crate-name v0.4.2`, timestamps), reveal `min(tick*2+8, total)` lines, end with `Build finished in N.NNs` when complete. TestRunner: pytest-style (`tests/test_auth.py::test_login PASSED [ 12%]`) ending in a green-summary line. Plain text only — NO lipgloss/ANSI in generators (App renders raw inside its own style).

- [ ] **Step 4: Run** `go test ./internal/tui/focus/ -v` → PASS. **Commit** — `feat(focus): fake htop/buildlog/testrunner generators`

---

### Task 6: Focus overlay wiring

**Files:**
- Modify: `internal/tui/app.go` — App fields, `/focus` registration (replace remaining hidden stub), `View()` short-circuit (app.go:809), key intercept in `Update` (app.go:453), tick.
- Test: `internal/tui/focus_wiring_test.go`

**Interfaces:**
- Consumes: `focus.RandomKind`, `focus.Render` (Task 5).
- Produces: App fields `focusActive bool`, `focusKind focus.Kind`, `focusTick int`, `focusSeed int64`; msg type `focusTickMsg struct{}`.

- [ ] **Step 1: Failing tests.** Drive the App via `Update` with real messages:
  - `/focus` submit → `focusActive` true, `View()` contains one of the fake markers ("Load average" / "Compiling" / "PASSED") and does NOT contain the track title.
  - Any `tea.KeyMsg` (e.g. rune 'x') while active → `focusActive` false, view back to normal.
  - `ctrl+c` while active → returns `tea.Quit` cmd.
  - `tea.MouseMsg` while active → still active.
  - `focusTickMsg` while active → `focusTick` incremented, another tick cmd returned; while inactive → no-op.

- [ ] **Step 2: Implement.**
  - Handler: `a.focusActive = true; a.focusKind = focus.RandomKind(rng); a.focusTick = 0; a.focusSeed = time.Now().UnixNano()` → return sentinel `"FOCUS_ON"`; in `handleSubmit`, on `"FOCUS_ON"` return the tick cmd. (Look at how existing sentinels map to cmds in `handleSubmit` app.go:1202 and mirror.)
  - Tick: `func focusTick() tea.Cmd { return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return focusTickMsg{} }) }`.
  - `Update`: FIRST branch after `tea.MouseMsg`: if `a.focusActive` — on `tea.KeyMsg`: `ctrl+c` → quit; anything else → `focusActive=false`, return; on `focusTickMsg` → `focusTick++`, return next tick cmd; swallow mouse.
  - `View()`: `if a.focusActive { return focus.Render(a.focusKind, a.width, a.height, rand.New(rand.NewSource(a.focusSeed)), a.focusTick) }` (use the App's stored width/height fields — grep for `tea.WindowSizeMsg` handling to find their names).
  - Playback untouched — do not pause/stop anything.

- [ ] **Step 3: Run** `go test ./internal/tui/ -run Focus -v -short` → PASS. **Commit** — `feat(focus): /focus fake work screen, any key returns`

---

### Task 7: core.MediaRemote port + TUI wiring (noop adapter)

**Files:**
- Create: `core/mediaremote.go`, `internal/adapters/mediakeys/noop.go`, `internal/adapters/mediakeys/mediakeys.go` (constructor: returns per-GOOS impl; this task: always noop)
- Modify: `internal/tui/app.go` + `cmd/wrkmon-go/main.go` (inject), `internal/config/config.go` (`MediaKeys bool \`toml:"media_keys"\``, default true)
- Test: `internal/tui/remote_wiring_test.go`

**Interfaces:**
- Produces (Tasks 8-9 implement against these EXACT types):

```go
// core/mediaremote.go
package core

import "time"

type RemoteCommand int

const (
    RemotePlayPause RemoteCommand = iota
    RemoteNext
    RemotePrev
    RemoteStop
)

type NowPlaying struct {
    Title    string
    Artist   string
    Duration time.Duration
    Position time.Duration
    Playing  bool
}

type MediaRemote interface {
    Commands() <-chan RemoteCommand
    Publish(np NowPlaying)
    Close() error
}
```

- `mediakeys.New(appName string) core.MediaRemote` — dispatches per GOOS; unknown/unsupported → `mediakeys.Noop{}` (exported struct with `ch chan core.RemoteCommand` closed-never; Commands returns nil-safe channel; Publish/Close no-ops).
- App: `listenRemote() tea.Cmd` reading one command → `remoteCmdMsg{cmd}`; Update handles it EXACTLY like the keyboard equivalents (Space/n/p/stop) then re-issues `listenRemote()`. Publish called at: track start, pause, resume, stop, auto-advance (grep the app for where `TogglePause`/`doNextTrack` complete and where track-started state lands; ONE helper `a.publishNowPlaying()` called from those points).

- [ ] **Step 1: Failing tests** — fake remote with a buffered channel injected into App: push `RemotePlayPause` → assert facade fake recorded TogglePause; `RemoteNext` → NextTrack; also assert `publishNowPlaying` composes `core.NowPlaying` from facade state (title/playing).
- [ ] **Step 2: Implement** port + noop + wiring; `media_keys=false` in config → always noop, no listener started.
- [ ] **Step 3: Run** `go test ./internal/tui/ -run Remote -v -short` + `go build ./...` → PASS. **Commit** — `feat(core): MediaRemote port with noop adapter and TUI wiring`

---

### Task 8: Linux MPRIS adapter

**Files:**
- Create: `internal/adapters/mediakeys/mpris_linux.go` (build tag `//go:build linux`), `internal/adapters/mediakeys/mprismap.go` (pure helpers, no build tag), update `mediakeys.go` dispatch (linux → MPRIS, fallback noop on bus error)
- Test: `internal/adapters/mediakeys/mprismap_test.go` (pure), `internal/adapters/mediakeys/mpris_linux_test.go` (Short-guarded real bus)
- Modify: `go.mod` (dep)

**Interfaces:**
- Consumes: `core.MediaRemote`, `core.NowPlaying` (Task 7).
- Produces: `newMPRIS(appName string) (core.MediaRemote, error)`; pure `func mprisMetadata(np core.NowPlaying) map[string]dbus.Variant` and `func playbackStatus(np core.NowPlaying) string` ("Playing"/"Paused"/"Stopped": Playing flag + zero-Title → "Stopped").

- [ ] **Step 1: Dep** (sandbox OFF): `go get github.com/godbus/dbus/v5@latest && go mod tidy`
- [ ] **Step 2: Failing pure tests** — metadata map: `xesam:title`, `xesam:artist` ([]string), `mpris:length` (int64 µs), `mpris:trackid` (dbus.ObjectPath, `/org/wrkmon/track/1`); status mapping table.
- [ ] **Step 3: Implement.** Bus name `org.mpris.MediaPlayer2.wrkmon_go`; export root iface (Identity "wrkmon", CanQuit/CanRaise false, no SupportedUriSchemes/MimeTypes needed but export empty arrays — some clients require the props) and Player iface: methods PlayPause/Play/Pause/Next/Previous/Stop → push mapped RemoteCommand into a buffered(8) channel, drop when full. Properties via `github.com/godbus/dbus/v5/prop` helper: PlaybackStatus, Metadata, Position (int64 µs), Rate/MinimumRate/MaximumRate 1.0, Volume 1.0, CanPlay/CanPause/CanGoNext/CanGoPrevious/CanControl true, CanSeek false (skip seek v1 — SetPosition/Seek methods present but no-op). Also export `org.freedesktop.DBus.Introspectable` (prop pkg + introspect pkg handle this).
  - `Publish`: update prop store (emits PropertiesChanged automatically via prop pkg `Set`); throttle: always update on Playing/Title change; update Position silently (`prop.EmitFalse` for Position — MPRIS spec says Position changes are NOT signalled).
  - Constructor: `dbus.SessionBus()` error → return error (dispatch falls back to noop + stderr warning once).
- [ ] **Step 4: Short-guarded bus test:** skip if `DBUS_SESSION_BUS_ADDRESS` unset; construct, then from the SAME test connect a second bus client, call `PlayPause` on our name, assert command arrives on `Commands()` within 2s; check `playerctl --player=wrkmon_go status` if playerctl present (skip otherwise).
- [ ] **Step 5: Run** `go test ./internal/adapters/mediakeys/ -v` (not -short, on this machine) → PASS. `go build ./...` → clean. **Commit** — `feat(mediakeys): Linux MPRIS adapter via godbus`

---

### Task 9: Windows hotkeys + macOS passthrough

**Files:**
- Create: `internal/adapters/mediakeys/hotkeys_windows.go` (`//go:build windows`)
- Modify: `mediakeys.go` dispatch (windows → hotkeys), `internal/adapters/mpv/mpv.go` `Play`/spawn args (darwin: add `--input-media-keys=yes` — find where the arg slice is built in `Play`/`spawn`; add via a per-GOOS helper file `internal/adapters/mpv/args_darwin.go` + `args_other.go` returning extra args, so no runtime.GOOS branching in mpv.go), `go.mod` (x/sys)
- Test: compile-only — `GOOS=windows go build ./...` and `GOOS=darwin go build ./...`

**Interfaces:**
- Consumes: `core.MediaRemote` (Task 7).
- Produces: `newHotkeys() (core.MediaRemote, error)` (windows file); `func extraMPVArgs() []string` (darwin: `["--input-media-keys=yes"]`; other: nil).

- [ ] **Step 1: Dep** (sandbox OFF): `go get golang.org/x/sys@latest && go mod tidy`
- [ ] **Step 2: Implement windows adapter:** goroutine with `runtime.LockOSThread()`; `RegisterHotKey(0, id, 0, vk)` for {0xB3 playpause, 0xB0 next, 0xB1 prev, 0xB2 stop} via `windows.NewLazySystemDLL("user32.dll")` procs (RegisterHotKey, GetMessageW, PostThreadMessageW, UnregisterHotKey); GetMessage loop: WM_HOTKEY (0x0312) → map id → channel push (drop if full); Close: `PostThreadMessageW(threadID, WM_QUIT(0x0012), 0, 0)` + UnregisterHotKey. Partial registration OK: keep what registered, log what failed. Publish = no-op.
- [ ] **Step 3: mpv darwin args** per the helper-file split above; unit test for `extraMPVArgs` presence under darwin is compile-tag-bound, so instead assert in `args_other.go` test that it returns nil on linux.
- [ ] **Step 4: Verify** `GOOS=windows go build ./... && GOOS=darwin go build ./... && go build ./... && go vet ./...` → all clean. **Commit** — `feat(mediakeys): Windows media hotkeys; macOS mpv media-key passthrough`

---

### Task 10: Kits, help, README, final sweep

**Files:**
- Modify: `context/kits/cavekit-focus-mode.md` (scope → fake work screen per spec; rewrite R2 "Minimal Visible Content" into "Fake work screen content" with acceptance criteria: overlay shows no track info; any key except ctrl+c dismisses; playback continues), `context/kits/cavekit-media-keys.md` (note Linux MPRIS full / Windows keys-only / macOS mpv passthrough reality), `context/kits/cavekit-equalizer.md` (add custom-bands requirement), `README.md` (features list + /eq /focus + media keys blurb + --version), help overlay text if commands listed there (grep `"/lyrics"` in app.go help strings to find it).
- Test: full suite.

- [ ] **Step 1: Kit + README + help edits.**
- [ ] **Step 2: Full verification:**

```bash
gofmt -l . | tee /dev/stderr | wc -l   # must be 0
go vet ./...
go test ./... -short
go test ./internal/adapters/mediakeys/ ./internal/adapters/mpv/ ./core/  # full, real bus+mpv
GOOS=windows go build ./... && GOOS=darwin go build ./...
go build -ldflags "-X main.version=dev-final" -o /tmp/wg ./cmd/wrkmon-go && /tmp/wg --version
```

- [ ] **Step 3: Live tmux verify (this machine):** launch, play a track, then: `playerctl --player=wrkmon_go play-pause` pauses+resumes; `playerctl --player=wrkmon_go next` advances queue; `/eq bass` → feedback + no error; `/eq show` lists; `/eq off`; `/focus` → fake screen (capture-pane shows htop/build/test content, NO track title), press key → player back, music still playing (position advanced).
- [ ] **Step 4: Commit** — `docs: revise focus/media-keys/eq kits to shipped reality` — then push all task commits: `git push origin main`.
