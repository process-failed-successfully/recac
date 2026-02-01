## 2026-01-24 - TUI Inline Validation
**Learning:** In terminal UIs (Bubble Tea), error states must be explicitly cleared on user input. Unlike web frameworks, there's no automatic "dirty" state handling.
**Action:** When adding validation to TUI inputs, always include a handler in the `Update` loop to reset error messages on `tea.KeyMsg`.

## 2026-02-01 - TUI Multiline Input Traps
**Learning:** Terminal chat interfaces often accidentally block multiline input (like pasting code) because `Enter` is hijacked for submission. Users expect `Shift+Enter` or similar, but terminals often don't distinguish modifiers well.
**Action:** Always implement an explicit, discoverable key binding (like `Alt+Enter`) for newlines in TUI chat inputs to support power users and code pasting.
