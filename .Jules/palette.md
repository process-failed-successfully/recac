## 2026-01-24 - TUI Inline Validation
**Learning:** In terminal UIs (Bubble Tea), error states must be explicitly cleared on user input. Unlike web frameworks, there's no automatic "dirty" state handling.
**Action:** When adding validation to TUI inputs, always include a handler in the `Update` loop to reset error messages on `tea.KeyMsg`.

## 2026-06-25 - [TUI Table Detail View Pattern]
**Learning:** `bubbles/table` lacks built-in "expand" functionality. Implementing a master-detail view requires manual state management (e.g., `showDetails` bool) and conditional rendering in the `View` method, treating the model as a simple router.
**Action:** When adding drill-down features to TUI tables, wrap the table in a parent model that manages the "view state" (list vs. details) and handles navigation keys (`Enter`/`Esc`) to toggle this state.
