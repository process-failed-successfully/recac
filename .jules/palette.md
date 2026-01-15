## 2025-12-28 - [TUI Feedback Gap]
**Learning:** Users often assume the app is frozen when long-running background tasks (like shell commands) execute without UI updates in a TUI.
**Action:** Always couple async command triggers with an immediate state update (e.g., `thinking = true`) and ensure the completion message (e.g., `shellOutputMsg`) explicitly clears that state.

## 2025-05-20 - Contextual Help in TUI
**Learning:** Users often get stuck in TUI submenus (like lists) because navigation keys (Up/Down/Esc) aren't explicitly shown in the footer help, which defaults to global keys.
**Action:** Implement a dynamic `keyMap` wrapper that adjusts the `ShortHelp` output based on the active UI mode (e.g., showing "select/back" in menus vs "send" in chat).

## 2025-10-26 - Standardizing TUI Help
**Learning:** Hardcoded help text (e.g., "press q to quit") in TUI footers is brittle and inconsistent.
**Action:** Always use `bubbles/help` with a defined `key.Binding` struct to ensure accessible, consistent, and auto-layout help menus across all TUI screens.
