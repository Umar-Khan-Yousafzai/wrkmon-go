---
created: "2026-04-14"
last_edited: "2026-04-14"
---

# Cavekit: Theme Picker

## Scope
An interactive screen for browsing and selecting a TUI theme, with live preview, keyboard navigation, commit, and cancel. Coexists with the existing direct-set-by-name command behavior.

## Requirements

### R1: Open the Picker
**Description:** The user opens the picker via the theme slash command with no arguments.
**Acceptance Criteria:**
- [ ] Invoking the theme slash command with no arguments opens the picker as a dedicated screen or overlay.
- [ ] Invoking the theme slash command with a theme name as argument bypasses the picker and sets that theme directly, preserving prior behavior.
- [ ] Invoking with an unknown theme name is rejected with a user-visible message and does not open the picker.

### R2: List Available Themes
**Description:** The picker shows all available themes the user can choose from.
**Acceptance Criteria:**
- [ ] Every theme selectable by name via the direct-set command is listed in the picker.
- [ ] The currently active theme is visually marked as the current selection when the picker opens.
- [ ] The list is presented in a stable, deterministic order across sessions.

### R3: Keyboard Navigation
**Description:** The user navigates the theme list with the keyboard.
**Acceptance Criteria:**
- [ ] Pressing the up arrow moves the highlighted selection one entry toward the top; at the first entry, behavior is deterministic (either stays or wraps) and documented in-screen or consistent across invocations.
- [ ] Pressing the down arrow moves the highlighted selection one entry toward the bottom with the same deterministic boundary behavior.
- [ ] Pressing Enter commits the highlighted theme.
- [ ] Pressing Esc cancels the picker.

### R4: Live Preview
**Description:** As the user moves through the list, the UI previews the highlighted theme.
**Acceptance Criteria:**
- [ ] Changing the highlighted entry updates visible UI colors or styling to reflect the highlighted theme within one rendered frame.
- [ ] The preview applies to at least the main surfaces of the picker screen itself (background, text, accent).
- [ ] The preview is not persisted until commit occurs.

### R5: Commit and Cancel
**Description:** Enter commits; Esc reverts.
**Acceptance Criteria:**
- [ ] On commit, the highlighted theme is applied to the entire TUI.
- [ ] On commit, the selected theme is written to user configuration and survives restart.
- [ ] On cancel, the theme in effect before opening the picker is restored.
- [ ] On cancel, no change is written to user configuration.

## Out of Scope
- Creating, editing, importing, or exporting custom themes.
- Per-screen or per-panel theme overrides.
- Auto-switching themes by time of day, system appearance, or ambient light.
- Fuzzy search or filtering within the picker.

## Cross-References
- See also: cavekit-focus-mode.md — a committed theme must apply consistently in focus mode.
- See also: cavekit-stealth-mode.md — stealth mode does not alter theme selection, but both affect user-visible presentation.

## Changelog
