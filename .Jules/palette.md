## 2026-01-24 - TUI Inline Validation
**Learning:** In terminal UIs (Bubble Tea), error states must be explicitly cleared on user input. Unlike web frameworks, there's no automatic "dirty" state handling.
**Action:** When adding validation to TUI inputs, always include a handler in the `Update` loop to reset error messages on `tea.KeyMsg`.

## 2025-05-20 - TUI Status Feedback
**Learning:** In TUI apps, subtle status changes (like "Thinking" vs "Generating") are easily missed if they share the exact same visual style.
**Action:** Use distinct text styles (italics/color) and specific verbs ("Generating") for different async states to reassure the user that the app is active.
