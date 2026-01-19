## 2025-05-18 - Namespace Collisions in Go TUI Styles
**Learning:** Unexported variables (like `helpStyle`) in a Go package are visible across all files in that package. I caused a build error by redefining `helpStyle` in `status_dashboard.go` while it already existed in `interactive.go`.
**Action:** Always prefix style variables with the component name (e.g., `statusHelpStyle`) or scope them locally within functions if they don't need to be package-global.
