# Media Keys + Equalizer + Focus Mode (fake shell) — Design

Date: 2026-07-14
Status: Approved (user, 2026-07-14)

## Goal

Bring three v1 features into wrkmon-go v2, zero-CGO throughout:

1. **Media keys** — hardware Play/Pause/Next/Prev control, cross-platform (Linux full, Windows functional, macOS partial).
2. **Equalizer** — mpv audio-filter EQ with named presets AND per-band custom gains.
3. **Focus mode** — v1-style fake work screen (`/focus`): fake htop / build log / test runner overlay; any key returns to player. This supersedes the earlier "minimal layout" scope in `context/kits/cavekit-focus-mode.md` (kit to be revised).

Bonus: `--version` CLI flag (version var exists, unused).

Out of scope: stealth mode (title override), sleep timer, theme picker, SMTC metadata on Windows, macOS queue-aware next/prev.

## 1. Media keys

### Architecture

- New port on core: `MediaRemote` interface — adapter publishes player state/metadata and delivers user commands back into the TUI event loop.
- Adapter package: `internal/adapters/mediakeys/` with per-GOOS files.
- Commands flow INTO the app as Bubble Tea messages (adapter → channel → `tea.Cmd` listener → same handlers as keyboard shortcuts). Never touch player/queue directly from the adapter goroutine.

```go
// core (new file core/mediaremote.go)
type RemoteCommand int // RemotePlayPause, RemoteNext, RemotePrev, RemoteStop
type NowPlaying struct {
    Title, Artist string
    Duration      time.Duration
    Position      time.Duration
    Playing       bool
}
type MediaRemote interface {
    Commands() <-chan RemoteCommand   // buffered; adapter drops if full
    Publish(np NowPlaying)            // no-op where unsupported
    Close() error
}
```

### Linux — MPRIS (full support)

- Dep: `github.com/godbus/dbus/v5` (pure Go).
- Session bus name `org.mpris.MediaPlayer2.wrkmon_go` (bus names: no hyphens); object `/org/mpris/MediaPlayer2`.
- Implement interfaces `org.mpris.MediaPlayer2` (Identity="wrkmon", CanQuit=false, CanRaise=false) and `org.mpris.MediaPlayer2.Player`:
  - Methods: `PlayPause`, `Play`, `Pause`, `Next`, `Previous`, `Stop`. (`Seek`/`SetPosition`: optional, implement if trivial — position is already tracked.)
  - Properties: `PlaybackStatus` (Playing/Paused/Stopped), `Metadata` (`xesam:title`, `xesam:artist`, `mpris:length` µs, `mpris:trackid`), `Position` (µs), `CanPlay/CanPause/CanGoNext/CanGoPrevious` = true, `CanSeek` = true.
  - Emit `org.freedesktop.DBus.Properties.PropertiesChanged` on state/track change. Throttle: publish on track change + play/pause + every poll tick only if position drifted >2s from linear expectation (avoid 1Hz signal spam; MPRIS clients interpolate Position).
- Session bus absent (headless/SSH) → adapter constructor returns a no-op remote + logged warning; app runs normally.

### Windows — RegisterHotKey (keys work, no SMTC)

- `golang.org/x/sys/windows` syscalls only. Dedicated goroutine with locked OS thread: `RegisterHotKey` for VK_MEDIA_PLAY_PAUSE (0xB3), VK_MEDIA_NEXT_TRACK (0xB0), VK_MEDIA_PREV_TRACK (0xB1), VK_MEDIA_STOP (0xB2), then `GetMessage` loop → WM_HOTKEY → command channel. `PostThreadMessage(WM_QUIT)` on Close.
- Registration failure (another app owns a key): log, continue with whichever registered.
- `Publish` = no-op (SMTC deferred).

### macOS — mpv passthrough (play/pause only)

- No wrkmon adapter (no-op remote). Instead: on darwin, mpv spawn adds `--input-media-keys=yes` — mpv claims Play/Pause hardware key; our 1Hz property polling picks up pause state changes so the TUI stays in sync.
- Documented limitation: Next/Prev not queue-aware on macOS.

### TUI wiring

- `tui.NewApp` gains the remote; `Init()` starts a `listenRemote` `tea.Cmd`. RemotePlayPause behaves exactly like Space, RemoteNext like `n`, RemotePrev like `p`.
- Publish points: track start, pause/resume, stop, track end/auto-advance.
- Config: flat key `media_keys = true` (opt-out), matching existing flat config style.

## 2. Equalizer

### mpv side

- Filter: FFmpeg `superequalizer` (18 fixed bands, params `1b`..`18b`, gain multiplier 0–20, 1.0 = unity). Applied live over IPC: `set_property af lavfi=[superequalizer=1b=X:2b=Y:...]` — exact syntax to be verified against mpv 0.37 during implementation (task includes a probe test against real mpv).
- `off` → `set_property af ""` (clear chain). EQ owns the whole `af` chain (nothing else uses it today).
- Reapply: after mpv respawn and on every new track load (mpv per-file filters persist across loadfile when set as property — verify; if they persist, apply once + after respawn only).

### Model

- `core/eq.go`: `EQState { Preset string; Gains [18]float64 }` (gains in dB, -12..+12, 0 = flat; converted to superequalizer multiplier via `math.Pow(10, db/20)`).
- Built-in presets (dB curves over the 18 bands, ~65Hz–20kHz): `flat`, `bass`, `treble`, `vocal`, `rock`, `pop`.
- Custom: `/eq band <1-18> <gain>` mutates current gains, preset label becomes `custom`.

### Commands (`/eq` namespace — currently stub-hidden, unhide)

- `/eq` or `/eq show` — statusline/flash listing preset + non-zero bands (compact single-line, matches existing command feedback style).
- `/eq <preset>` — apply preset.
- `/eq off` — bypass (clears af), keeps stored gains.
- `/eq band <n> <gain>` — set band n gain in dB (validate n∈[1,18], gain∈[-12,12]).
- Persistence: `eq_preset`, `eq_gains` (18 floats), `eq_enabled` in config.toml; restored and applied on startup when something is playing/loads.

## 3. Focus mode — v1 fake shell

### Behavior (v1 parity + refresh)

- `/focus` → full-screen overlay replaces the whole TUI view; playback, timers, auto-advance continue.
- Overlay type: random pick per invocation — `htop`, `buildlog`, `testrunner`.
- **Any key** → dismiss, back to previous view (exceptions: Ctrl+C quits app as everywhere else). Mouse events ignored (no accidental dismiss from wheel).
- htop variant refreshes its numbers every 2s (tick) so it looks live; buildlog appends a line every 1–3s until "build" completes then idles; testrunner similar. Static generation acceptable for first pass IF ticking complicates; ticking is the target.
- Content generators ported from v1 `wrkmon/ui/screens/focus.py` + fake process names from v1 `StealthManager.FAKE_PROCESS_NAMES` (node-inspector, webpack-dev-srv, vite-hmr-watch, eslint-daemon, tsc-watch, pytest-runner, cargo-watch, go-build-srv, rust-analyzer, prettier-fmt).
- No now-playing info anywhere on the overlay (that's the point).

### Implementation

- `internal/tui/focus/` package: pure generator funcs `Htop(w, h int, seed rand source) string`, `BuildLog(...)`, `TestRunner(...)` — deterministic given a seed (testable), sized to terminal.
- App model gains `focusActive bool` + `focusKind` + tick cmd; `View()` short-circuits to focus render when active. Key handler intercepts all keys when active.
- `/focus` namespace unhidden.

## 4. --version flag

- `wrkmon-go --version` / `-v` / `version` → print `wrkmon-go <version>` (ldflags-stamped var) and exit 0, BEFORE any config/dep checks.

## Error handling summary

- Missing session bus / hotkey registration failure / unsupported OS → degrade to no-op, never block startup.
- EQ apply failure (mpv IPC error) → flash error, state kept, retry on next track.
- All adapter goroutines shut down cleanly on quit (Close called from app teardown).

## Testing

- Unit: EQ preset→multiplier conversion, band validation, config round-trip; focus generators (golden-ish shape assertions: line counts, width bounds, contains expected headers); remote command routing (fake MediaRemote channel → app messages); MPRIS property map construction (pure func, no bus).
- Integration (Linux, this machine): real session-bus MPRIS test behind `-short` guard (`dbus-send`/godbus client asserts PlayPause reaches channel); real mpv af probe test behind `-short` guard.
- Live tmux verification: play track → `playerctl play-pause/next` works; `/eq bass` audibly changes + `/eq show` correct; `/focus` shows fake screen, any key returns; `--version` prints.
- Windows path: compile-checked via `GOOS=windows go build`; runtime untested here (documented).

## Kit revision

- `context/kits/cavekit-focus-mode.md`: scope rewritten to fake-work-screen semantics (this spec).
- `context/kits/cavekit-media-keys.md`, `cavekit-equalizer.md`: mark Linux-first reality + custom bands addition; acceptance criteria updated where they conflict with this spec.

## Dependencies

- `github.com/godbus/dbus/v5` (new)
- `golang.org/x/sys` (new, windows build only)
- Network required for `go get` (sandbox off).
