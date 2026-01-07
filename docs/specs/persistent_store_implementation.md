# Persistent Store Implementation Plan

## Overview
This document outlines the plan to introduce a persistent external store (PostgreSQL) to `recac`. This is a critical requirement for supporting **Orchestrator Mode** in Kubernetes, enabling multi-pod state sharing, reliable concurrency, and data persistence across pod restarts.

## Goals
1.  **Multi-Backend Support**: Refactor `internal/db` to support both SQLite (local/default) and PostgreSQL (cluster/production).
2.  **Kubernetes Readiness**: Ensure `recac` agents and managers running in separate pods can share state (features, signals, locks) via the external DB.
3.  **Concurrency Safety**: Leverage database transactions and row-level locking for robust multi-agent coordination.
4.  **Zero-Config Local Dev**: Maintain SQLite as the default for seamless local development without requiring a DB server.

## Implementation Steps

### Phase 1: Dependency & Configuration
1.  **Add Driver**: Add `github.com/lib/pq` (or `pgx`) to `go.mod`.
2.  **Config Schema**: Update `config.yaml` and `viper` bindings to support:
    *   `store.type`: `sqlite` (default) or `postgres`
    *   `store.connection_string`: Connection URL (e.g., `postgres://user:pass@host:5432/recac?sslmode=disable`) or file path for SQLite.

### Phase 2: Refactor `internal/db`
1.  **Factory Pattern**: Create a `NewStore(config)` factory function in `factory.go` that returns the `Store` interface.
2.  **Postgres Implementation**: Create `postgres.go` implementing `Store`.
    *   **Migrations**: Port SQLite migrations to PostgreSQL syntax (handle `AUTOINCREMENT` vs `SERIAL`, `DATETIME` types).
    *   **Locking**: Implement `AcquireLock` using Postgres Advisory Locks or row-level locking (`SELECT FOR UPDATE`) for better reliability than file-based logic.
    *   **JSON Handling**: Use `JSONB` for `project_features` content to allow efficient querying in the future (though `TEXT` is sufficient for parity).

### Phase 3: Application Updates
1.  **`recac start`**: Update session initialization to read `store.type` from config/env (`RECAC_STORE_TYPE`) and instantiate the correct store.
2.  **`agent-bridge`**: Update to support connecting to Postgres.
    *   *Challenge*: `agent-bridge` currently takes a DB path arg.
    *   *Solution*: Update it to accept a connection string or read from env `RECAC_DB_URL`.
3.  **Docker/K8s**:
    *   Update `Dockerfile` to include necessary libraries (if any, though pure Go is fine).
    *   Update Helm charts (`deploy/helm/recac`) to support `postgres` configuration (env vars).

### Phase 4: Signal & Directives Persistence
1.  **Manager Directives**: Move `manager_directives.txt` to a DB signal (`MANAGER_DIRECTIVES`) or a dedicated table.
    *   Update Manager Prompt to write to this signal via `agent-bridge`.
    *   Update Coding Agent to read from this signal (via `agent-bridge` or injected context).
2.  **Fix Rejection Flow**:
    *   Implement `agent-bridge signal REJECTED true`.
    *   Update `session.go` to respect this signal and clear `COMPLETED`/`QA_PASSED`.

## Technical Details

### Database Schema (Postgres)

```sql
CREATE TABLE IF NOT EXISTS observations (
    id SERIAL PRIMARY KEY,
    agent_id TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS signals (
    key TEXT PRIMARY KEY,
    value TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS project_features (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    content JSONB NOT NULL, -- Using JSONB for Postgres
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS file_locks (
    path TEXT PRIMARY KEY,
    agent_id TEXT NOT NULL,
    lock_type TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP NOT NULL
);
```

### Migration Strategy
*   **Auto-Migration**: The application will auto-migrate the schema on startup (create tables if not exist).
*   **Data Migration**: No automatic data migration from SQLite to Postgres is planned. Orchestrator mode assumes a fresh environment or manually managed persistence.

### Data Retention & Self-Cleaning
To prevent the database from growing indefinitely, especially in long-running or high-churn Orchestrator environments, the following retention policies will be implemented:

1.  **Signals & Locks (TTL)**:
    *   **Locks**: Already have an `expires_at` field. A background cleaner (or access check) will delete expired rows.
    *   **Signals**: Most are ephemeral state (e.g., `TRIGGER_QA`). We will add a `cleanup_signals` routine that removes non-critical signals older than *N* hours (configurable, default 24h). Critical signals like `PROJECT_SIGNED_OFF` should persist until manual cleanup or project deletion.

2.  **Observations (History limit)**:
    *   **Hard Limit**: Implement a rolling window for `observations`.
    *   **Strategy**: On every *Nth* insertion (or via background job), delete observations older than *X* days OR keep only the last *Y* records per `agent_id`.
    *   **Config**: `db.history.retention_days` (default: 7) and `db.history.max_entries` (default: 10,000).

3.  **Project Features**:
    *   **Snapshotting**: Currently, we only store the *latest* version (ID=1). This is self-cleaning by design. If we enable history/versioning later, we must enforce a max version count.

4.  **Implementation**:
    *   Add a `Cleanup()` method to the `Store` interface.
    *   Call `Cleanup()` periodically from the `Orchestrator` or `Session` loop (e.g., once per hour or at session start/end).

## Verification Plan
1.  **Unit Tests**: Add tests for `PostgresStore` (requires Dockerized Postgres for testing or mocking).
2.  **Smoke Test (Local)**: Verify SQLite still works (regression test).
3.  **Smoke Test (K8s)**: Deploy to K8s with a Postgres sidecar/service, configure `RECAC_DB_URL`, and run a full cycle (Init -> Code -> QA -> Manager).

## Future Considerations
*   **Artifact Storage**: Store generic files (logs, diffs) in DB or S3 instead of just `observations`.
*   **Events**: Use Postgres `NOTIFY/LISTEN` for real-time signaling between Manager and Agents instead of polling.
