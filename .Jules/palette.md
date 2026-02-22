## 2026-01-24 - TUI Inline Validation
**Learning:** In terminal UIs (Bubble Tea), error states must be explicitly cleared on user input. Unlike web frameworks, there's no automatic "dirty" state handling.
**Action:** When adding validation to TUI inputs, always include a handler in the `Update` loop to reset error messages on `tea.KeyMsg`.

## 2026-02-17 - Bubble Tea Table Styling
**Learning:** The `bubbles/table` component accepts strings with ANSI escape codes in cells, allowing for rich, inline styling of data without needing custom cell renderers.
**Action:** Use `lipgloss.Style.Render()` directly on cell content strings to apply colors or bolding to specific table data.
