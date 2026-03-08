A fantastic, bold improvement: **Job History Search/Filtering in the CLI and API**.
Currently, `--list-jobs` with `--history` gets ALL jobs. `--export-jobs` also gets ALL jobs.
If you have a large database (many weeks of jobs), this is not scalable and hard to read.
Adding the ability to filter by:
- `status` (e.g., Failed, Completed)
- `since` (e.g., last 24h, 7d)
- `search` (text search in Summary or ID)
- `limit` (max number of results)

This is highly requested in any production orchestration tool.
It requires changes in:
1. `api.go`: Modify `GET /jobs` and `GET /jobs/export` to parse query parameters (`status`, `since`, `search`, `limit`) and pass them to a new function `GetFilteredJobs(filter JobFilter)`.
2. `orchestrator.go`:
   - Define `JobFilter` struct.
   - Add `GetFilteredJobs(filter JobFilter) []JobInfo`.
   - Update `SQLitePersistence` to support the filter natively in the SQL query! (Or we can just filter the fetched ones if native is too complex, but native is better). Wait, `GetJobs` in `SQLitePersistence` currently returns `limit`. Let's add `SearchJobs(filter JobFilter)`.
3. `cmd/orchestrator`:
   - Add flags to `--list-jobs`: `--filter-status`, `--filter-since`, `--filter-search`, `--limit`.
   - Update `listJobs` to send these as query params.

Wait, since `GetJobs` on `SQLitePersistence` is part of the `Persistence` interface, adding `SearchJobs` changes the interface.
Let's check `internal/orchestrator/persistence.go`:
