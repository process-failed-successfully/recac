1. **Analyze `recac/internal/db/postgres.go`**
   - The file is relatively small. I will write some new tests and augment the existing ones to cover error paths for `NewPostgresStore`, `migrate`, `UpdateFeatureStatus`, `AcquireLock`, `ReleaseLock`, `GetActiveLocks`, `Cleanup`.
2. **Review test files and write tests**
   - Use `append` pattern or overwrite carefully.
3. **Analyze `recac/internal/db/sqlite.go`**
   - The file `sqlite.go` implements the same interface but for SQLite.
   - Look at `recac/internal/db/sqlite.go` and implement `sqlite_test.go` from scratch.
4. **Complete pre-commit steps to ensure proper testing, verification, review, and reflection are done.**
5. **Check overall coverage to ensure >80% coverage and Submit**
