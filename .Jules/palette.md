## 2025-12-28 - [TUI Feedback Gap]
**Learning:** Users often assume the app is frozen when long-running background tasks (like shell commands) execute without UI updates in a TUI.
**Action:** Always couple async command triggers with an immediate state update (e.g., `thinking = true`) and ensure the completion message (e.g., `shellOutputMsg`) explicitly clears that state.
