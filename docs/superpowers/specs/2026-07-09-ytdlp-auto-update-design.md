# Design: yt-dlp Startup Version Check + Auto-Update

**Date:** 2026-07-09
**Status:** Approved
**Problem:** yt-dlp breaks as YouTube changes; stale copies (e.g. a 2024.04 system install) silently kill stream-URL extraction. The Linux/macOS installer only warns the user to install yt-dlp, so most users run a system copy that `yt-dlp -U` refuses to self-update. wrkmon-go must check on EVERY startup and fix itself automatically.

## Goals

1. Every app start verifies yt-dlp freshness in the background (never delays TUI launch).
2. When the active binary cannot self-update (system PATH copy), migrate once to a wrkmon-managed copy that can, then hot-swap to it without restart.
3. Zero root, zero package-manager interference: never touch the system binary.

## Non-goals

- No pinned/minimum-version table; "latest official release" is the target.
- No UI beyond toasts; no changes to search/playback logic.

## Managed locator tier

New tier in `ytdlp.Locate`, precedence becomes:

1. Config override (unchanged)
2. **Managed: `<DataDir>/bin/yt-dlp[.exe]`** (i.e. `~/.local/share/wrkmon-go/bin/`) — user-writable, wrkmon-owned; treated as self-updatable (`Bundled=true` semantics, `Source="managed"`)
3. Bundled next to the wrkmon binary (unchanged)
4. System PATH (unchanged)
5. Error (unchanged)

The managed dir path comes from the caller (wired from `config.DataDir()`) so the locator stays testable with a temp dir.

## Startup flow

After the Bubble Tea program starts (a `tea.Cmd` fired from `Init`, gated by config `auto_update_ytdlp`, default `true`):

- **Self-updatable binary (managed or bundled):** run `yt-dlp -U` (it no-ops when current). Silent when already current; toast `yt-dlp updated → <version>` when an update was applied.
- **System-PATH binary:** one-time migration — download the official standalone build for this OS from `https://github.com/yt-dlp/yt-dlp/releases/latest/download/<asset>` (`yt-dlp_linux` / `yt-dlp_macos` / `yt-dlp.exe`; linux-arm64 uses `yt-dlp_linux_aarch64`) into the managed dir, then hot-swap the live client to it. Toast `yt-dlp <version> installed (managed copy)`. Every subsequent start takes the fast `-U` path because the managed tier now wins.
- **Any failure** (offline, GitHub down, verify failed): warning toast; the app keeps using the existing binary. Next start retries.

The check runs on every start (no throttle) — `-U` against a current binary is a single fast HTTP HEAD-style check by yt-dlp itself; the GitHub download happens at most once per machine.

## Download safety

- Download to `<managed-dir>/.yt-dlp.tmp`, `chmod 0755` (non-Windows), verify by executing `--version` (output must parse as a version-looking string), then atomic `os.Rename` onto the final name. Never overwrite in place; a failed verify deletes the temp file.
- HTTP client timeout 5 minutes; follow redirects (GitHub latest/download is a redirect chain).

## Hot-swap

`ytdlp.Client` gains a `sync.RWMutex` around `binPath`/`bundled`; all command builders read through an accessor. New method `Relocate(path string, selfUpdatable bool)` swaps atomically — in-flight commands finish on the old path, subsequent calls use the new one.

## Surface changes

- `ytdlp.Client.EnsureLatest(ctx) (msg string, updated bool, err error)` — the one entry point implementing the branch logic above (self-update vs migrate). `/update-ytdlp` switches to it, so the manual command also gains migration ability.
- `internal/tui`: `YtDlpAutoUpdateMsg{Info string, Updated bool, Err error}` fired by the startup `tea.Cmd`; handler shows the appropriate toast (nothing when `!Updated && Err == nil`).
- Config: `auto_update_ytdlp` (bool, default `true`, opt-out).
- Locator: managed tier as above; `Locate` signature gains the managed-dir argument.

## Error handling summary

| Failure | Behavior |
|---------|----------|
| offline / GitHub unreachable | warn toast, keep current binary, retry next start |
| downloaded file fails `--version` | delete temp, warn toast, keep current binary |
| managed dir not writable | warn toast with the path, keep current binary |
| `-U` reports "can't self-update" unexpectedly | treat as failure branch (warn) |

## Testing

- Locator: managed tier wins over bundled/system with a temp managed dir; empty managed dir falls through (table test).
- Client: `Relocate` swap visible to next command (accessor test); `EnsureLatest` migration path against `httptest.Server` serving a fake binary (a shell script echoing a version) → verifies download→chmod→verify→rename→relocate chain; failure paths (404, bad binary) leave the original binPath untouched.
- Config default test (`auto_update_ytdlp` true when absent).
- TUI: message handler toast logic (updated / silent / error) — table test if cheap, else covered by existing manual-check conventions.

## Cross-platform

Per-GOOS asset name; `.exe` suffix and no chmod on Windows; works with all three locator outcomes on every OS. Zero CGO preserved.
