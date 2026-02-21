## 2026-01-24 - TUI Inline Validation
**Learning:** In terminal UIs (Bubble Tea), error states must be explicitly cleared on user input. Unlike web frameworks, there's no automatic "dirty" state handling.
**Action:** When adding validation to TUI inputs, always include a handler in the `Update` loop to reset error messages on `tea.KeyMsg`.

## 2026-06-03 - Static Dashboard Refresh
**Learning:** Static HTML dashboards served without WebSocket support require manual refresh controls to be usable for monitoring dynamic data.
**Action:** Always include a manual "Refresh" button and a "Last Updated" timestamp when implementing simple dashboards to avoid user frustration with stale data.
