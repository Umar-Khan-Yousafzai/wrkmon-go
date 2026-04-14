---
created: "2026-04-14"
last_edited: "2026-04-14"
---

# Cavekit: Equalizer

## Scope
A curated set of audio equalizer presets the user can select to shape playback tone. Covers the preset list, selection command, immediate application to audio, and persistence across restarts.

## Requirements

### R1: Curated Preset List
**Description:** The equalizer offers a fixed, small set of named presets.
**Acceptance Criteria:**
- [ ] The available presets are exactly: flat, bass-boost, vocal-boost, treble-boost.
- [ ] Each preset has a distinct, deterministic frequency-shaping effect when applied.
- [ ] An additional disabled state ("off") represents no EQ processing and is selectable alongside the presets.

### R2: Query Current Preset
**Description:** The user can view which preset is currently active.
**Acceptance Criteria:**
- [ ] Invoking the equalizer namespace with no arguments reports the currently active preset or indicates that the equalizer is off.
- [ ] The reported value matches the most recent successful selection from R3 or the persisted value from R5.

### R3: Select a Preset
**Description:** The user can set the active preset by name.
**Acceptance Criteria:**
- [ ] Invoking the equalizer namespace with a valid preset name from R1 activates that preset and reports confirmation.
- [ ] Invoking the equalizer namespace with "off" disables equalizer processing and reports confirmation.
- [ ] Invoking with an unknown preset name is rejected with a user-visible message and leaves the current preset unchanged.

### R4: Immediate Application
**Description:** A selection takes effect without waiting for track boundaries.
**Acceptance Criteria:**
- [ ] Selecting a preset while a track is playing changes the audible tone of that track without stopping or restarting playback.
- [ ] Selecting a preset while nothing is playing causes the selection to apply to the next track that plays.
- [ ] Switching presets back-to-back leaves only the most recent selection active.

### R5: Persistence Across Restarts
**Description:** The active preset survives application restart.
**Acceptance Criteria:**
- [ ] After selecting a preset and restarting the application, the same preset is active on startup.
- [ ] After selecting "off" and restarting, the equalizer starts disabled.
- [ ] The persisted value is stored in user configuration that survives restart.

## Out of Scope
- User-defined custom presets or per-band sliders.
- Per-track or per-playlist EQ assignment.
- Real-time spectrum or visualizer display.
- Room correction, loudness normalization, or compression beyond what a preset inherently applies.
- Preset import/export.

## Cross-References
- See also: cavekit-focus-mode.md — the equalizer selection command must remain usable while focus mode is active.

## Changelog
