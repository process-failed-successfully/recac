## 2026-01-24 - TUI Inline Validation
**Learning:** In terminal UIs (Bubble Tea), error states must be explicitly cleared on user input. Unlike web frameworks, there's no automatic "dirty" state handling.
**Action:** When adding validation to TUI inputs, always include a handler in the `Update` loop to reset error messages on `tea.KeyMsg`.

## 2026-02-06 - [TUI Context Persistence]
**Learning:** In TUI applications, clearing the screen ("resetting") can disorient users by removing instructional context. A "soft reset" that restores the initial welcome/help state is superior to a blank slate.
**Action:** When implementing "Clear" commands in TUIs, always restore the initial onboarding state or quick-help text.
