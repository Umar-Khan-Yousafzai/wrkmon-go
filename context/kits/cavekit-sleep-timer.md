---
created: "2026-04-14"
last_edited: "2026-04-14"
revision: 2
---

# Cavekit: Sleep Timer

## Scope
User-initiated countdown that halts playback after a chosen duration. Covers starting, inspecting, cancelling, and resetting the timer, plus the halt action applied when the timer elapses. Lives for the duration of one app session.

## Requirements

### R1: Start a Timer
**Description:** The user can start a sleep timer with a duration expressed in whole minutes.
**Acceptance Criteria:**
- [ ] Invoking the sleep namespace with a positive integer argument starts a timer for that many minutes.
- [ ] Only one sleep timer is active at a time; starting a new one replaces any running timer.
- [ ] A non-positive or non-integer argument is rejected with a user-visible message and no timer is started.
- [ ] Starting a timer produces a user-visible confirmation including the scheduled end time or remaining duration.

### R2: Inspect Remaining Time
**Description:** The user can query how much time remains on the active timer.
**Acceptance Criteria:**
- [ ] A status query while a timer is active reports remaining time with at least minute precision.
- [ ] A status query when no timer is active reports that no timer is set.
- [ ] The reported remaining time decreases monotonically as real time passes.

### R3: Cancel or Reset the Timer
**Description:** The user can cancel an active timer, or reset it to a new duration.
**Acceptance Criteria:**
- [ ] A cancel action on an active timer stops it and produces a user-visible confirmation.
- [ ] A cancel action when no timer is active produces a user-visible message stating there is nothing to cancel and does not error.
- [ ] Starting a timer while one is already running replaces the previous timer with the new duration (covered by R1) and the previous end time is discarded.

### R4: Halt Playback on Elapse
**Description:** When the timer reaches zero, playback is halted.
**Acceptance Criteria:**
- [ ] On elapse, the current track stops producing audio.
- [ ] On elapse, the user is informed via a visible indication that the sleep timer fired.
- [ ] After elapse, no further queued tracks begin playing automatically.
- [ ] After elapse, the timer is considered inactive and a status query reports no active timer.
- [ ] After elapse, a subsequent play action from any source (slash command, keyboard shortcut, or media key) resumes the current track from its halted position and does not re-arm the timer.

### R5: Session-Scoped Lifetime
**Description:** Timer state exists only for the current session.
**Acceptance Criteria:**
- [ ] An active timer persists across track changes within the same session.
- [ ] An active timer is not restored after the application is closed and reopened; a fresh session starts with no timer.
- [ ] No timer state is written to durable user configuration.

## Out of Scope
- Sub-minute granularity (seconds, fractional minutes).
- Scheduling a timer to fire at a wall-clock time instead of a relative duration.
- Multiple concurrent timers.
- Timer recovery or restoration after application restart.
- Fade-out or volume-ramp behavior as the timer elapses.
- "End of current track" or "end of queue" halt modes.

## Cross-References
- See also: cavekit-media-keys.md — the halt action must coexist with media-key-initiated play/pause without conflict.

## Changelog
