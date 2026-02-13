## 2026-01-24 - TUI Inline Validation
**Learning:** In terminal UIs (Bubble Tea), error states must be explicitly cleared on user input. Unlike web frameworks, there's no automatic "dirty" state handling.
**Action:** When adding validation to TUI inputs, always include a handler in the `Update` loop to reset error messages on `tea.KeyMsg`.

## 2026-02-13 - CLI Step Indicators
**Learning:** For multi-step wizards in Bubble Tea, user orientation is significantly improved by adding explicit step indicators (e.g., "Step X/Y") styled distinctly with Lipgloss.
**Action:** Standardize wizard headers to include a subtle step counter alongside the main title.
