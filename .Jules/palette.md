## 2026-01-24 - TUI Inline Validation
**Learning:** In terminal UIs (Bubble Tea), error states must be explicitly cleared on user input. Unlike web frameworks, there's no automatic "dirty" state handling.
**Action:** When adding validation to TUI inputs, always include a handler in the `Update` loop to reset error messages on `tea.KeyMsg`.

## 2026-02-05 - TUI Command Discovery & Key Bindings
**Learning:** Users often search for commands by intent (description) rather than exact name (e.g., "exit" for "/quit"). Fuzzy or description-based filtering significantly improves discoverability in CLI tools.
**Action:** Always include description matching in command palette filters.

**Learning:** Bubble Tea's `textarea` defaults `Enter` to insert newline. For chat apps, `Enter` is expected to send. Rebinding `InsertNewline` to `Alt+Enter` prevents conflict and allows multiline input.
**Action:** Explicitly set `ta.KeyMap.InsertNewline.SetKeys("alt+enter")` in chat interfaces.
