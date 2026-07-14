---
created: "2026-04-14"
last_edited: "2026-07-14"
---

# Cavekit: Media Keys

## Scope
Operating-system-level media control integration so standard play/pause, next, and previous media keys (and equivalent software controls surfaced by the OS) act on the player while the app is running. Covers per-platform availability, graceful degradation, and optional now-playing metadata exposure.

## Platform Reality (as shipped)
Coverage is intentionally Linux-first; the other two platforms are partial by design, not a gap to be silently assumed away. Acceptance criteria below are annotated per-platform where behavior differs.

- **Linux — full support.** `internal/adapters/mediakeys` implements MPRIS over session D-Bus (`github.com/godbus/dbus/v5`, pure Go, zero CGO) as `org.mpris.MediaPlayer2.wrkmon_go`. Transport (play/pause/next/previous/stop) and now-playing metadata (title/artist/position/duration) are both exposed; standard MPRIS clients (`playerctl`, desktop shells) work out of the box.
- **Windows — keys work, no now-playing surface.** `RegisterHotKey` (via `golang.org/x/sys/windows`, no CGO) claims the four media hotkeys and drives transport. There is no SMTC (System Media Transport Controls) integration, so no metadata is exposed to the OS — R5 does not apply on Windows.
- **macOS — passthrough, play/pause only.** There is no wrkmon-authored adapter; wrkmon's `MediaRemote` is a no-op on darwin. Instead, mpv itself is launched with `--input-media-keys=yes`, so mpv claims the hardware play/pause key directly, and wrkmon's periodic player-state poll picks up the resulting pause/resume so the TUI stays in sync. Next/previous hardware keys are **not** wired to wrkmon's queue on macOS (mpv has no concept of wrkmon's queue) — R1's next/previous ACs do not hold on macOS. No metadata is exposed (R5 does not apply).

## Requirements

### R1: Media Key Actions Control Playback
**Description:** Standard OS media controls drive the player's transport.
**Acceptance Criteria:**
- [ ] A play/pause media key toggles between playing and paused states of the current track — holds on Linux, Windows, and macOS.
- [ ] A next media key advances to the next track in the queue when one exists — holds on Linux and Windows; not implemented on macOS (see Platform Reality).
- [ ] A previous media key returns to the prior track when history allows — holds on Linux and Windows; not implemented on macOS (see Platform Reality).
- [ ] Each action takes effect whether or not the app has terminal focus.

### R2: Cross-Platform Coverage
**Description:** Integration is attempted on Linux, macOS, and Windows using each platform's standard mechanism.
**Acceptance Criteria:**
- [ ] On Linux, integration is attempted using the desktop-standard media control interface (MPRIS via D-Bus) and succeeds on systems that provide a session bus.
- [ ] On macOS, integration is attempted via mpv's own `--input-media-keys=yes` media-key claim (not a wrkmon-authored OS adapter) and succeeds on supported versions for play/pause only.
- [ ] On Windows, integration is attempted using the OS-provided global media hotkey mechanism (`RegisterHotKey`) and succeeds on supported versions.

### R3: Graceful Degradation
**Description:** When OS integration is unavailable, the app continues to function normally.
**Acceptance Criteria:**
- [ ] If registration with the OS mechanism fails, the application starts and runs without crashing.
- [ ] If integration is unavailable, no user-visible error dialog or blocking message is shown on startup.
- [ ] All non-media-key playback controls continue to work when media key integration is unavailable.
- [ ] A record of the unavailability is retrievable for diagnostic purposes without being surfaced to the normal user.

### R4: Enabled by Default, User-Disable Option
**Description:** Integration is on by default and can be turned off via user configuration.
**Acceptance Criteria:**
- [ ] With no user configuration changes, media key integration is attempted on first launch.
- [ ] A configuration flag exists that, when set to a disabled value, prevents the app from registering any OS media key integration for the session.
- [ ] The disabled setting persists across application restarts.

### R5: Now-Playing Metadata Exposure (Optional)
**Description:** When the OS mechanism supports it, the app exposes current track information. As shipped, this is Linux-only (see Platform Reality) — Windows has no SMTC integration and macOS has no wrkmon-authored adapter, so neither exposes metadata.
**Acceptance Criteria:**
- [ ] On Linux (MPRIS), when a track is active, the track title is exposed via the `Metadata` property (`xesam:title`).
- [ ] On Linux (MPRIS), when known, the track artist/uploader is exposed (`xesam:artist`).
- [ ] On Linux (MPRIS), when no track is active, no stale track metadata remains exposed.
- [ ] Failure to expose metadata does not prevent transport controls (R1) from working, on any platform.
- [ ] On Windows and macOS, the absence of metadata exposure is expected behavior, not a defect — R1 transport (Windows fully, macOS play/pause only) still functions.

## Out of Scope
- Custom global hotkeys beyond standard media keys.
- Exposure of artwork, lyrics, chapter markers, or seek scrubber position beyond basic title/artist.
- Remote control from mobile devices or network endpoints.
- Media key bindings while the app is fully closed.
- Platforms other than Linux, macOS, and Windows.
- Windows SMTC (System Media Transport Controls) metadata/thumbnail integration — Windows coverage is transport-only (R1), not metadata (R5).
- A wrkmon-authored macOS media-key adapter — macOS relies entirely on mpv's own `--input-media-keys=yes` passthrough for play/pause.

## Cross-References
- See also: cavekit-sleep-timer.md — a media-key play action after the sleep timer has fired must behave consistently with the halted state.
- See also: cavekit-stealth-mode.md — stealth mode may suppress now-playing metadata exposure described in R5.

## Changelog
- 2026-07-14: Added "Platform Reality" section and annotated R1/R2/R5 ACs with per-platform caveats (Linux full MPRIS; Windows RegisterHotKey transport only, no SMTC; macOS mpv `--input-media-keys=yes` passthrough, play/pause only, no wrkmon adapter). See design spec §1.
