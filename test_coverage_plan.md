1. **Explore `cmd/orchestrator/artifacts.go` coverage:** Identify missing error path coverage in `cmd/orchestrator/artifacts_test.go` and implement it.
2. **Review other `cmd/orchestrator` packages with low coverage:** Pick `apply_pipeline.go` and `diagnose.go` to add error path / failure scenario unit tests.
3. **Add unit tests to hit error branches:** Create table-driven tests that mock out negative HTTP responses, failed network calls, or invalid filepaths to trigger the `exitFunc(1)` branches in `artifacts.go` and others.
4. **Complete pre-commit steps to ensure proper testing, verification, review, and reflection are done.**
5. **Verify coverage has improved to >80%:** Run `make cover` and parse output.
