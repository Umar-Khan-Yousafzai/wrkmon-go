---
created: "2026-04-14"
last_edited: "2026-04-14"
---

# Cavekit: Stealth Mode

## Scope
A toggle that overrides the terminal window title with a neutral, user-configurable string and optionally suppresses now-playing exposure through OS media integration. Persists in user configuration.

## Requirements

### R1: Toggle via Slash Command
**Description:** The user enables and disables stealth mode through a dedicated slash command.
**Acceptance Criteria:**
- [ ] Invoking the stealth slash command while stealth is off turns it on and reports confirmation.
- [ ] Invoking the stealth slash command while stealth is on turns it off and reports confirmation.
- [ ] Invoking the stealth slash command reports the resulting state to the user.

### R2: Terminal Title Override
**Description:** When stealth is on, the terminal window title is replaced with a neutral string.
**Acceptance Criteria:**
- [ ] When stealth is enabled, the terminal window title is set to the configured neutral string.
- [ ] When stealth is disabled, the app stops overriding the title and any stealth-specific title is cleared.
- [ ] Title overrides issued by the app for track changes or other events do not occur while stealth is enabled.

### R3: Configurable Neutral String
**Description:** The neutral title string is user-configurable with a safe default.
**Acceptance Criteria:**
- [ ] The default neutral string when no user configuration is set is "Terminal".
- [ ] The user can configure a different neutral string, which persists across restarts.
- [ ] An empty or whitespace-only configured value falls back to the default without error.

### R4: Persistence Across Restarts
**Description:** Stealth on/off state survives restart.
**Acceptance Criteria:**
- [ ] If stealth is enabled at shutdown, it is enabled on next startup and the terminal title is the neutral string.
- [ ] If stealth is disabled at shutdown, it is disabled on next startup.
- [ ] Stealth state is stored in user configuration.

### R5: Metadata Suppression (Optional)
**Description:** When stealth is active, now-playing metadata surfaced to the OS is suppressed.
**Acceptance Criteria:**
- [ ] While stealth is enabled, any track title or artist that would otherwise be exposed via OS media integration is not exposed or is replaced with a neutral placeholder.
- [ ] When stealth is disabled, normal metadata exposure resumes for subsequent track changes.
- [ ] Suppression failure (for example, because the OS integration is unavailable) does not prevent the title override in R2.

## Out of Scope
- Process name or command-line obfuscation.
- Audio muting or playback halting on stealth toggle.
- Hiding the application from process listings, task managers, or system monitors.
- Encrypting, hiding, or redacting user configuration, queue, or history files.
- A user-configurable list of alternate titles that rotate.

## Cross-References
- See also: cavekit-media-keys.md — metadata suppression in R5 modifies the metadata exposure defined there.
- See also: cavekit-theme-picker.md — stealth mode does not change theme; both concern user-visible presentation.

## Changelog
