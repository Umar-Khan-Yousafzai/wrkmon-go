---
created: "2026-04-14"
last_edited: "2026-07-14"
---

# Cavekit: Equalizer

## Scope
A curated set of audio equalizer presets, plus per-band custom gain control, the user can select to shape playback tone. Covers the preset list, custom bands, selection command, immediate application to audio, and persistence across restarts.

> Revision note (2026-07-14): shipped as an 18-band `superequalizer` (mpv/FFmpeg audio filter) rather than a fixed set of opaque presets — see R6 (new) for the custom-bands requirement added on top of the original preset scope. See `docs/superpowers/specs/2026-07-14-mediakeys-eq-focus-design.md` §2.

## Requirements

### R1: Curated Preset List
**Description:** The equalizer offers a fixed, small set of named presets.
**Acceptance Criteria:**
- [ ] The available presets are exactly: flat, bass, pop, rock, treble, vocal.
- [ ] Each preset has a distinct, deterministic frequency-shaping effect when applied (a fixed dB curve across 18 bands, converted to a filter gain multiplier).
- [ ] An additional disabled state ("off") represents no EQ processing and is selectable alongside the presets.

### R2: Query Current Preset
**Description:** The user can view which preset (or custom band configuration) is currently active.
**Acceptance Criteria:**
- [ ] Invoking the equalizer namespace with no arguments (or with `show`) reports the currently active preset — or `custom` plus the non-zero bands (R6) — or indicates that the equalizer is off.
- [ ] The reported value matches the most recent successful selection from R3/R6 or the persisted value from R5.

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
**Description:** The active preset (or custom bands) survives application restart.
**Acceptance Criteria:**
- [ ] After selecting a preset and restarting the application, the same preset is active on startup.
- [ ] After setting custom band gains (R6) and restarting, the same 18 band gains are active on startup, labeled `custom`.
- [ ] After selecting "off" and restarting, the equalizer starts disabled (stored gains/preset are retained but not applied until re-enabled).
- [ ] The persisted value (preset name, all 18 band gains, enabled flag) is stored in user configuration that survives restart.

### R6: Per-Band Custom Gains
**Description:** The user can set an individual band's gain directly, independent of the curated presets in R1, for finer control than a preset provides.
**Acceptance Criteria:**
- [ ] The equalizer exposes exactly 18 fixed frequency bands (numbered 1–18, low to high), each independently settable.
- [ ] Setting a single band's gain (in dB, within a bounded range) takes effect immediately, exactly like R4, without stopping or restarting playback.
- [ ] Setting any individual band's gain switches the reported active state to `custom` (R2), regardless of which preset was active before.
- [ ] Setting a band gain outside the supported range, or for a band number outside 1–18, is rejected with a user-visible message and leaves the current gains unchanged.
- [ ] Custom band gains persist across restart exactly as R5 describes for presets.

## Out of Scope
- Per-track or per-playlist EQ assignment.
- Real-time spectrum or visualizer display.
- Room correction, loudness normalization, or compression beyond what a preset or custom band configuration inherently applies.
- Preset import/export, or saving a custom configuration under a user-given name.
- More or fewer than 18 bands, or a graphical/slider-based band editor (band gains are set one at a time by number, not via a visual UI).

## Cross-References
- See also: cavekit-focus-mode.md — the equalizer selection command is NOT reachable while the focus-mode overlay is active (focus mode dismisses on any keypress, including the `/` that would start typing an `/eq` command); the user must dismiss focus mode first.

## Changelog
- 2026-07-14: Added R6 (per-band custom gains, 18 bands); updated R1 preset names to the shipped list (flat/bass/pop/rock/treble/vocal); R2/R5 extended to cover the `custom` state and band-gain persistence; removed "no per-band sliders" from Out of Scope (superseded by R6); corrected the focus-mode cross-reference (equalizer is not reachable while the fake-screen overlay is up). See design spec §2.
