# Design: Seek Suite + Infinite-Scroll Search + App Window

**Date:** 2026-07-09
**Status:** Approved
**Scope:** Three user-facing gaps in v2.0.0-alpha.2: weak/undiscoverable time seek, hard 10-result search limit, and no way to run wrkmon-go as its own desktop app.

## Goals

1. Full seek control: multi-step key jumps, exact-position `/seek` command, clickable/draggable seekbar, and discoverability hints.
2. Search results load continuously as the user scrolls (infinite scroll) instead of stopping at `results_per_page`.
3. `wrkmon-go window` launches the TUI in its own dedicated window with desktop/Start-Menu integration (Approach A: terminal-launcher wrapper — chosen over a webview shell or a Fyne terminal widget to keep zero-CGO, zero new deps, and native terminal performance).

## Non-goals

- No native graphical UI (buttons/album art). The window hosts the existing Bubble Tea TUI.
- No change to playback engine, queue, playlists, downloads, or lyrics.
- No bundled terminal emulator; we launch one already on the system.

---

## 1. Seek suite

### Port and adapter

`ports.Player` gains:

```go
SeekTo(seconds float64) error      // absolute position
SeekPercent(pct float64) error     // 0–100, absolute-percent
```

mpv adapter implements them via IPC: `["seek", n, "absolute"]` and `["seek", pct, "absolute-percent"]`. The existing relative `Seek(seconds float64)` is unchanged.

### Keyboard

| Key | Action | Condition |
|-----|--------|-----------|
| `left` / `right` | seek −5s / +5s | prompt empty (existing behavior) |
| `shift+left` / `shift+right` | seek −30s / +30s | always (no editing conflict) |

Every seek (key, command, or mouse) shows a toast: `⏩ 1:23 / 3:41` (current / total after the seek).

### `/seek` command

Registered in `internal/tui/commands` with autocomplete. Accepted arguments:

| Input | Meaning |
|-------|---------|
| `/seek 1:23` | absolute mm:ss (also `h:mm:ss`) |
| `/seek 83` | absolute seconds |
| `/seek 50%` | percent of duration |
| `/seek +30` / `/seek -30` | relative seconds |

Parser is a pure function `ParseSeek(arg string) (SeekSpec, error)` where `SeekSpec{Kind: absolute|percent|relative, Value float64}`. Parse failure → error toast with usage string. No track playing → toast "nothing playing". Absolute/percent values are clamped to `[0, duration]` by mpv itself; no extra clamping logic.

### Clickable / draggable seekbar

- Bubble Tea program starts with `tea.WithMouseCellMotion()` when config `mouse = true` (new key, default `true`). `false` restores terminal-native text selection.
- The statusbar (and now-playing view) records the screen-cell bounds of its progress bar during `View()` (start column, end column, row).
- `tea.MouseMsg` click inside the bounds → fraction = (x − start) / width → `SeekPercent(fraction*100)`.
- Drag: while left button held and motion stays in the bar's row band, apply seeks live, throttled to at most 4 applied seeks/second (drop intermediate positions; always apply the release position).
- Mouse wheel over result/queue/playlist lists scrolls them (comes free with mouse mode and is required for infinite scroll ergonomics).

### Discoverability

- Help overlay gains the new keys and `/seek` forms.
- One-time toast on first playback of a session: `←/→ seek 5s · Shift ±30s · /seek 1:23 · click the bar`. Shown once per app run, not persisted.

---

## 2. Infinite-scroll search

yt-dlp has no search cursor — `ytsearchN:` always returns the first N. Strategy: accumulate and refetch with a growing N, appending only unseen results.

### Behavior

- Initial search fetches `results_per_page` (default 10) exactly as today — fast first paint.
- When the cursor (or mouse-wheel scroll) moves within 3 rows of the end of the loaded list, and no fetch is in flight, and the list is not exhausted → background fetch `ytsearch<len+25>:query`, then append results whose `VideoID` is not already in the list (YouTube ordering shifts between calls; dedupe is mandatory).
- Exhaustion: a fetch that yields zero new VideoIDs marks the query exhausted.
- Hard cap: `max_search_results` config key, default 100. Reaching the cap renders the end-of-results marker.
- Bottom sentinel row, one of: `… loading more` · `— end of results —` · `fetch failed — scroll to retry` (a further scroll-to-bottom retries once per gesture).

### State

Search view state grows: `exhausted bool`, `loadingMore bool`, `lastFetchErr error`. The result slice stays `[]core.SearchResult` — no schema change to `core`.

### Cache

The SQLite search cache (1h TTL) stores the full accumulated list per query, overwritten after each successful append. A cache hit restores the whole accumulated list (may exceed 10 rows immediately). Table schema unchanged.

### Config

- `results_per_page` is reinterpreted: initial fetch size and scroll page-step only — no longer the total limit.
- New: `max_search_results` (int, default 100).

### Cost note

Each refetch re-downloads metadata from index 0 (yt-dlp limitation). Flat-playlist search is metadata-only and the cap bounds it: worst case for one query ≈ fetches of 10, 35, 60, 85, 100+25→cap. Accepted trade-off; no workaround exists short of the YouTube API.

---

## 3. App window (`wrkmon-go window`)

### Subcommand

`wrkmon-go window` (alias flag `--window`) resolves a terminal emulator and re-executes itself inside it in app mode, then exits. The child process is plain `wrkmon-go` — the TUI is unaware it runs in a dedicated window.

### Terminal resolution

1. Config override: `[window] terminal = "kitty"` (or any name from the table) + optional `extra_args = []string`.
2. `auto` (default): probe `$PATH` in order — `kitty`, `alacritty`, `wezterm`, `ghostty`, `foot`, `cosmic-term`, `gnome-terminal`, `konsole`, `xterm`. Windows: `wt`, else fall back to `cmd /c start` (conhost). macOS: `open -a` the first of iTerm/Alacritty/kitty, else AppleScript Terminal.
3. Nothing found → exit 1 with a message listing supported terminals and the config override.

### Launch table (table-driven, one row per terminal)

| Terminal | Invocation |
|----------|-----------|
| kitty | `kitty --class=wrkmon-go --title=wrkmon -e <self>` |
| alacritty | `alacritty --class wrkmon-go --title wrkmon -e <self>` |
| wezterm | `wezterm start --class wrkmon-go -- <self>` |
| ghostty | `ghostty --class=wrkmon-go -e <self>` |
| foot | `foot --app-id=wrkmon-go --title=wrkmon <self>` |
| cosmic-term | `cosmic-term -- <self>` |
| gnome-terminal | `gnome-terminal --title=wrkmon -- <self>` |
| konsole | `konsole --separate -e <self>` |
| xterm | `xterm -class wrkmon-go -T wrkmon -e <self>` |
| wt (Windows) | `wt --title wrkmon <self>` |

`<self>` = `os.Executable()`. The `--class`/`--app-id` value `wrkmon-go` must match `StartupWMClass` in the desktop file so docks group the window under the wrkmon icon.

### Desktop integration

- `assets/wrkmon-go.desktop`: `Exec=wrkmon-go window`, `Icon=wrkmon-go`, `StartupWMClass=wrkmon-go`, `Terminal=false`, `Categories=AudioVideo;Audio;Player;`.
- `assets/icon.png` (new asset, 256×256).
- `scripts/install.sh` installs both to `~/.local/share/applications/` and `~/.local/share/icons/hicolor/256x256/apps/`, then runs `update-desktop-database` if present. Uninstall removes them.
- `.deb` packaging includes desktop file + icon at the system paths.
- Windows installer adds a Start-Menu shortcut invoking `wrkmon-go.exe window`.
- macOS: install.sh creates a minimal `~/Applications/wrkmon.app` bundle (Info.plist + launcher script exec'ing `wrkmon-go window`) so the app appears in Launchpad/Spotlight.

Cross-platform is a hard requirement: all three features work on Linux, macOS, and Windows; OS-specific code sits behind `runtime.GOOS` or per-OS files.

---

## Error handling summary

| Failure | Behavior |
|---------|----------|
| `/seek` parse error | toast with usage |
| seek with nothing playing | toast "nothing playing" |
| mpv IPC error on seek | toast with error (same pattern as volume) |
| fetch-more fails | sentinel row shows failure; scroll-to-bottom retries |
| no terminal found (`window`) | exit 1, actionable message |
| terminal spawns but exits non-zero instantly | stderr passthrough + exit 1 |

## Testing

- **Unit:** `ParseSeek` (every accepted form + garbage + empty), mpv `SeekTo`/`SeekPercent` against the existing mock-IPC harness, search accumulator (dedupe on overlapping refetch, exhaustion on zero-new, cap enforcement, error→retry state), terminal resolver (table-driven with fake `$PATH` dirs), seekbar hit-region math (click x → percent, bounds edges).
- **Manual checklist:** click + drag seekbar in kitty and gnome-terminal; wheel-scroll past 10 results and observe append; `/seek 50%`; shift+arrows while typing in prompt; launch from the .desktop entry on Pop!_OS (dock icon groups); `window` with config override; Windows `wt` launch.
- Zero-CGO preserved (no new build constraints). No DB migration.

## File touch map

| Area | Files |
|------|-------|
| Ports | `internal/ports/ports.go` |
| mpv | `internal/adapters/mpv/mpv.go` (+ test) |
| Seek parse + command | `internal/tui/commands/` (+ test) |
| Keys, mouse, toasts, help | `internal/tui/app.go`, `internal/tui/components/statusbar.go` |
| Search accumulate | `internal/tui/app.go`, `internal/adapters/ytdlp/ytdlp.go` (+ test) |
| Config | `internal/config/config.go` |
| Window subcommand | `cmd/wrkmon-go/main.go`, new `internal/window/` (+ test) |
| Assets/packaging | `assets/`, `scripts/install.sh`, deb + Windows installer scripts |
