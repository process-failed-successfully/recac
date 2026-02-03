## 2026-01-24 - TUI Inline Validation
**Learning:** In terminal UIs (Bubble Tea), error states must be explicitly cleared on user input. Unlike web frameworks, there's no automatic "dirty" state handling.
**Action:** When adding validation to TUI inputs, always include a handler in the `Update` loop to reset error messages on `tea.KeyMsg`.

## 2026-02-03 - TUI Discoverability in Idle States
**Learning:** Users often miss keyboard shortcuts in TUIs because they lack visible chrome (buttons/menus). The "empty" or "idle" status area is prime real estate for teaching these interactions without clutter.
**Action:** Whenever a TUI has a status line that sits empty when idle, inject a subtle hint (low contrast text) listing the primary shortcuts (e.g., "Press / for menu").
