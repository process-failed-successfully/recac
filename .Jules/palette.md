## 2026-01-24 - TUI Inline Validation
**Learning:** In terminal UIs (Bubble Tea), error states must be explicitly cleared on user input. Unlike web frameworks, there's no automatic "dirty" state handling.
**Action:** When adding validation to TUI inputs, always include a handler in the `Update` loop to reset error messages on `tea.KeyMsg`.

## 2026-01-24 - TUI Onboarding
**Learning:** Empty states in TUIs can be intimidating. A structured, formatted welcome message acts as "onboarding" to guide users on available commands.
**Action:** Use rich text styling and bullet points in the initial view to explain key interactions (chat vs commands vs shell).
