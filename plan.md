1. **Identify the inconsistency:**
   The Web UI dashboard (`internal/orchestrator/webui.go`) allows modifying the dependencies of a job via the "Set Deps" button. However, the TUI dashboard (`internal/tui/dashboard.go`) does not have this functionality, even though the CLI does (`--update-deps-job`, `--set-deps`). I will implement the ability to edit a job's dependencies from within the TUI.
2. **Add "Set Deps" functionality to the TUI:**
   - Add a keybinding (e.g., `D` for dependencies) in the main view of the TUI.
   - When `D` is pressed on a selected job, capture the current dependencies, pre-fill them, and switch to a new view/input field for editing the dependencies.
   - Send the PUT request to `/jobs/{id}/dependencies` to update them upon confirmation.
   - Refresh the UI.
3. **Update tests:**
   - Add a test for the `updateDependencies` command that the TUI will execute.
   - Add a test case checking that the `D` key transitions to the right state.
