# Palette's Journal - Critical UX/A11y Learnings

## 2024-05-23 - Redundant Status & Contrast
**Learning:** Duplicate status indicators (top bar + bottom badge) created visual noise. The top bar's grey (`244`) had poor contrast in light mode.
**Action:** Consolidate status to one clear location and use `AdaptiveColor` for better readability.

## 2025-12-28 - [TUI Feedback Gap]
**Learning:** Users often assume the app is frozen when long-running background tasks (like shell commands) execute without UI updates in a TUI.
**Action:** Always couple async command triggers with an immediate state update (e.g., `thinking = true`) and ensure the completion message (e.g., `shellOutputMsg`) explicitly clears that state.
