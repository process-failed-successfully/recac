## 2024-05-23 - SQL Injection Fix
**Vulnerability:** Found `fmt.Sprintf` used to construct SQL queries with `NOT IN (%s)` clause in `internal/db/sqlite.go` and `internal/db/postgres.go`. While the input was currently hardcoded strings, this pattern is risky and could lead to SQL injection if the input becomes dynamic.
**Learning:** `database/sql` does not natively support slice arguments for `IN` clauses, leading developers to use string concatenation/formatting.
**Prevention:** Always use parameterized queries. For `IN` clauses with dynamic slice lengths, generate the appropriate number of placeholders (e.g., `?, ?, ?` or `$1, $2, $3`) dynamically and pass the slice as variadic arguments (`args...`).
