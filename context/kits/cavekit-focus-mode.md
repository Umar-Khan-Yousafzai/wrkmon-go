---
created: "2026-04-14"
last_edited: "2026-07-14"
---

# Cavekit: Focus Mode

## Scope
A full-screen fake "work" overlay, toggled via a dedicated slash command, that disguises the terminal as unrelated dev tooling (a process monitor, a build log, or a test runner) while music keeps playing underneath. Covers entering the overlay, the fabricated content it shows, dismissing it, and continuity of playback and app state while it is up.

> Revision note (2026-07-14): this kit originally scoped focus mode as a minimal now-playing layout (queue/history/log panels hidden, track info + progress shown). The shipped feature is a v1-parity disguise screen instead — the overlay shows no now-playing information at all, and is dismissed by *any* keypress rather than the same command that opened it. R2 below is rewritten accordingly; R1/R3/R4 are corrected where they conflicted with this reality. See `docs/superpowers/specs/2026-07-14-mediakeys-eq-focus-design.md` §3.

## Requirements

### R1: Entry via Slash Command
**Description:** The user enters focus mode through a dedicated slash command; the command is not how the user exits (see R3).
**Acceptance Criteria:**
- [ ] Invoking the focus slash command while not in focus mode enters focus mode, replacing the entire screen with a fake work overlay.
- [ ] The overlay kind (process monitor / build log / test runner) is chosen at random each time the command is invoked.
- [ ] The command is available from any view/state where slash commands are normally accepted.

### R2: Fake Work Screen Content
**Description:** The overlay fabricates plausible dev-tool output and exposes nothing about what is actually playing.
**Acceptance Criteria:**
- [ ] The overlay shows no track title, artist, playback position, or any other now-playing information anywhere on screen.
- [ ] The overlay shows no queue panel, history panel, or status bar from the real UI — only fabricated content (process table, build output, or test output) plus made-up process/tool names, not the real app's data.
- [ ] The process-monitor variant's numbers visibly refresh on a tick (so it reads as a live monitor, not a static screenshot).
- [ ] The build-log and test-runner variants reveal additional fabricated lines over time, eventually reaching a "completed"/"passed" idle state.

### R3: Dismissal on Any Key
**Description:** Any keypress — not a repeat of the entry command — closes the overlay; the app's global quit key still quits.
**Acceptance Criteria:**
- [ ] While the overlay is active, pressing any key other than the app's quit key (Ctrl+C) dismisses the overlay and returns to the exact view that was showing before entry, and does nothing else (no seek/pause/volume/navigation side effect from that keypress).
- [ ] Ctrl+C quits the whole application while the overlay is active, exactly as it does outside focus mode.
- [ ] Mouse events (wheel, click) are ignored while the overlay is active — they neither dismiss it nor act on the hidden UI underneath.

### R4: Playback and State Continuity
**Description:** Entering or dismissing the overlay never affects playback or the state of the view underneath it.
**Acceptance Criteria:**
- [ ] Music playing before entry continues playing, unpaused, while the overlay is up, and playback position keeps advancing.
- [ ] Any active app timer/tick (playback polling, auto-advance to the next track) keeps functioning while the overlay is up.
- [ ] Upon dismissal, the previously visible view, its selection, and the queue/history contents are exactly as they were before entry — nothing about app state changed while the overlay was hidden.
- [ ] Volume and playing/paused state are unaffected by entering or dismissing the overlay itself (independent of any dismiss keypress that happens to be a shortcut key elsewhere — see R3, the dismiss keypress performs no other action).

## Out of Scope
- Persisting focus mode across application restarts (it always starts inactive on launch).
- Real system data: all process-monitor/build-log/test-runner content is fabricated, never sourced from the actual OS or this app's build/test state.
- User-configurable overlay choice — the kind is randomized, not selectable.
- A separate "always on top" or windowing behavior (the terminal owns windowing).
- Automatic entry into focus mode based on inactivity, time of day, or OS-level window-focus loss.

## Cross-References
- See also: cavekit-theme-picker.md — theme changes must apply correctly to the default layout; the fake overlay itself is intentionally theme-independent (plain text, no app styling — that's part of the disguise).
- See also: cavekit-equalizer.md — because R3 dismisses on any key, the equalizer command is NOT reachable while the overlay is active; the user must dismiss focus mode first, then run `/eq`.

## Changelog
- 2026-07-14: Scope and R1–R4 rewritten from "minimal now-playing layout" to shipped "fake work screen" behavior (any-key dismissal, no now-playing info, playback/state continuity). See design spec §3.
