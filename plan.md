1. **Optimize `cmd/agent-bridge/main.go`**: Replace `strings.ToLower(f.Status) != "done"` with `!strings.EqualFold(f.Status, "done")`.
2. **Optimize `pkg/e2e/scenarios/sql_parser.go`**: Replace `strings.ToLower(t) == logicalType` with `strings.EqualFold(t, logicalType)`.
3. **Run Pre-Commit Checks**: Run tests and linting to ensure no regressions.
4. **Create PR**: Submit the changes with a descriptive PR title and message.
