## 2026-01-24 - TUI Inline Validation
**Learning:** In terminal UIs (Bubble Tea), error states must be explicitly cleared on user input. Unlike web frameworks, there's no automatic "dirty" state handling.
**Action:** When adding validation to TUI inputs, always include a handler in the `Update` loop to reset error messages on `tea.KeyMsg`.

## 2026-01-25 - Path Expansion in CLI Wizards
**Learning:** CLI users expect `~` to expand to home directory. Go's `os.Stat` does not handle this automatically, leading to frustrating "Path does not exist" errors even when the path is valid in the shell.
**Action:** Always wrap file path inputs with an `expandPath` helper that handles `~` before validation or usage.
