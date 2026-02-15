## 2026-01-24 - TUI Inline Validation
**Learning:** In terminal UIs (Bubble Tea), error states must be explicitly cleared on user input. Unlike web frameworks, there's no automatic "dirty" state handling.
**Action:** When adding validation to TUI inputs, always include a handler in the `Update` loop to reset error messages on `tea.KeyMsg`.

## 2026-05-22 - TUI Table Selection Logic
**Learning:** When using `bubbles/table`, relying on `SelectedRow()` cell content for state logic is brittle if the content is formatted (e.g., with icons or colors).
**Action:** Always parse or sanitize the cell content (e.g., `strings.ToLower`, `strings.Contains`) or, better yet, use the table cursor index to look up the original data source.
