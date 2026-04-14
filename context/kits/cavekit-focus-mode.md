---
created: "2026-04-14"
last_edited: "2026-04-14"
---

# Cavekit: Focus Mode

## Scope
A minimal layout that hides ancillary panels and shows only the current track and progress, toggled via a dedicated slash command. Covers entering, exiting, and preservation of controls and underlying state while in focus mode.

## Requirements

### R1: Toggle via Slash Command
**Description:** The user enters and exits focus mode through a dedicated slash command.
**Acceptance Criteria:**
- [ ] Invoking the focus slash command while not in focus mode enters focus mode.
- [ ] Invoking the focus slash command while in focus mode returns to the default layout.
- [ ] The command is available from the default layout and from focus mode itself.

### R2: Minimal Visible Content
**Description:** Focus mode hides non-essential UI and shows only now-playing information.
**Acceptance Criteria:**
- [ ] While in focus mode, the queue panel is not visible.
- [ ] While in focus mode, the history panel is not visible.
- [ ] While in focus mode, any log or debug panel is not visible.
- [ ] While in focus mode, the currently playing track's title and playback progress are visible.
- [ ] While in focus mode, the status bar remains visible.

### R3: Keyboard Shortcuts Preserved
**Description:** Standard playback shortcuts continue to work while focus mode is active.
**Acceptance Criteria:**
- [ ] The play/pause shortcut toggles playback while in focus mode.
- [ ] The seek-forward and seek-backward shortcuts work while in focus mode.
- [ ] The volume up and volume down shortcuts work while in focus mode.
- [ ] The next-track and previous-track shortcuts work while in focus mode.

### R4: State Preservation on Exit
**Description:** Exiting focus mode returns the user to the layout and selections they had before.
**Acceptance Criteria:**
- [ ] Upon exiting focus mode, the queue panel reappears with the same contents it had on entry.
- [ ] Upon exiting focus mode, the history panel reappears with the same contents it had on entry.
- [ ] Upon exiting focus mode, the currently selected item in any list is the same as it was on entry.
- [ ] Playback position, volume, and playing/paused state are unaffected by entering or exiting focus mode.

## Out of Scope
- Persisting focus mode across application restarts.
- A separate "always on top" or windowing behavior (the terminal owns windowing).
- Custom focus layouts, multiple focus variants, or user-configurable visible elements.
- Automatic entry into focus mode based on inactivity or time of day.
- Full-screen takeover of the terminal beyond hiding internal panels.

## Cross-References
- See also: cavekit-theme-picker.md — theme changes must apply correctly in both the default layout and focus mode.
- See also: cavekit-equalizer.md — the equalizer command must function while focus mode is active.

## Changelog
