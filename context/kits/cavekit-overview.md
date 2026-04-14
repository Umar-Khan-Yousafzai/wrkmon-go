---
created: "2026-04-14"
last_edited: "2026-04-14"
---

# Cavekit Overview

## Project
wrkmon-go — Go + Bubble Tea TUI YouTube audio player (v2.x rewrite of Python wrkmon)

This overview indexes the remaining v2.x feature kits that build on the v2.0.0-alpha.2 MVP (search, queue, playback, history, playlists, downloads, lyrics, repeat/shuffle, auto-advance, config persistence).

## Domain Index
| Domain | File | Summary | Status |
|--------|------|---------|--------|
| Sleep Timer | cavekit-sleep-timer.md | Session-scoped countdown that halts playback on elapse. | Drafted |
| Media Keys | cavekit-media-keys.md | OS-level media control integration with graceful degradation. | Drafted |
| Equalizer | cavekit-equalizer.md | Curated EQ presets selectable via slash command, persisted. | Drafted |
| Focus Mode | cavekit-focus-mode.md | Minimal now-playing layout toggled via slash command. | Drafted |
| Theme Picker | cavekit-theme-picker.md | Interactive theme picker with live preview and commit. | Drafted |
| Stealth Mode | cavekit-stealth-mode.md | Terminal title override and optional metadata suppression. | Drafted |

## Cross-Reference Map
| Domain A | Interacts With | Interaction Type |
|----------|---------------|------------------|
| Sleep Timer | Media Keys | Halt-on-elapse must coexist with media-key play/pause without conflict. |
| Stealth Mode | Media Keys | Stealth suppresses the now-playing metadata that media keys would otherwise expose. |
| Theme Picker | Focus Mode | Committed theme must apply consistently in both default and focus layouts. |
| Focus Mode | Equalizer | Equalizer slash command must remain usable while focus mode is active. |
| Theme Picker | Stealth Mode | Both shape user-visible presentation but operate on independent concerns. |

## Dependency Graph
```
Sleep Timer ── coordinates-with ──> Media Keys <── suppresses-metadata-of ── Stealth Mode
                                        │
                                        └── (independent of UX kits below)

Theme Picker ── applies-within ──> Focus Mode ── hosts ──> Equalizer (command still usable)
```

No kit depends on another for correctness; interactions are coordination points, not prerequisites. All six domains can be planned and implemented independently and in parallel.

## Coverage Summary
- Kits: 6
- Total requirements: 29 (Sleep Timer 5, Media Keys 5, Equalizer 5, Focus Mode 4, Theme Picker 5, Stealth Mode 5)
- Every requirement carries testable acceptance criteria (see individual kits).
