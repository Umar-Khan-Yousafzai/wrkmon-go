---
created: "2026-04-14"
last_edited: "2026-04-14"
---

# Cavekit: Media Keys

## Scope
Operating-system-level media control integration so standard play/pause, next, and previous media keys (and equivalent software controls surfaced by the OS) act on the player while the app is running. Covers per-platform availability, graceful degradation, and optional now-playing metadata exposure.

## Requirements

### R1: Media Key Actions Control Playback
**Description:** Standard OS media controls drive the player's transport.
**Acceptance Criteria:**
- [ ] A play/pause media key toggles between playing and paused states of the current track.
- [ ] A next media key advances to the next track in the queue when one exists.
- [ ] A previous media key returns to the prior track when history allows.
- [ ] Each action takes effect whether or not the app has terminal focus.

### R2: Cross-Platform Coverage
**Description:** Integration is attempted on Linux, macOS, and Windows using each platform's standard mechanism.
**Acceptance Criteria:**
- [ ] On Linux, integration is attempted using the desktop-standard media control interface and succeeds on systems that provide it.
- [ ] On macOS, integration is attempted using the system's media-remote mechanism and succeeds on supported versions.
- [ ] On Windows, integration is attempted using the OS-provided global media hotkey mechanism and succeeds on supported versions.

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
**Description:** When the OS mechanism supports it, the app exposes current track information.
**Acceptance Criteria:**
- [ ] When supported by the platform integration and a track is active, the track title is exposed to the OS.
- [ ] When supported and known, the track artist or uploader is exposed.
- [ ] When no track is active, no stale track metadata remains exposed.
- [ ] Failure to expose metadata does not prevent transport controls (R1) from working.

## Out of Scope
- Custom global hotkeys beyond standard media keys.
- Exposure of artwork, lyrics, chapter markers, or seek scrubber position beyond basic title/artist.
- Remote control from mobile devices or network endpoints.
- Media key bindings while the app is fully closed.
- Platforms other than Linux, macOS, and Windows.

## Cross-References
- See also: cavekit-sleep-timer.md — a media-key play action after the sleep timer has fired must behave consistently with the halted state.
- See also: cavekit-stealth-mode.md — stealth mode may suppress now-playing metadata exposure described in R5.

## Changelog
