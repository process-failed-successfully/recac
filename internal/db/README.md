# Database Schema Documentation

This project uses a local SQLite database (`.recac.db`) as the primary persistent storage layer. This document outlines the schema for all tables.

## Tables

### 1. `observations`

Stores historical interactions, events, or facts recorded by agents.

| Column       | Type     | Constraints               | Description                                             |
| :----------- | :------- | :------------------------ | :------------------------------------------------------ |
| `id`         | INTEGER  | PRIMARY KEY AUTOINCREMENT | Unique identifier for each observation.                 |
| `agent_id`   | TEXT     | NOT NULL                  | The ID/Role of the agent that produced the observation. |
| `content`    | TEXT     | NOT NULL                  | The actual content/body of the observation.             |
| `created_at` | DATETIME | DEFAULT CURRENT_TIMESTAMP | Timestamp when the observation was created.             |

---

### 2. `signals`

Stores key-value pairs representing lifecycle signals or state flags (e.g., `COMPLETED`, `QA_PASSED`, `BLOCKER`).

| Column       | Type     | Constraints               | Description                                 |
| :----------- | :------- | :------------------------ | :------------------------------------------ |
| `key`        | TEXT     | PRIMARY KEY               | The name of the signal.                     |
| `value`      | TEXT     |                           | The value associated with the signal.       |
| `created_at` | DATETIME | DEFAULT CURRENT_TIMESTAMP | Timestamp when the signal was last updated. |

---

### 3. `project_features`

Stores the authoritative feature list for the project as a JSON blob.

| Column       | Type     | Constraints                | Description                                                    |
| :----------- | :------- | :------------------------- | :------------------------------------------------------------- |
| `id`         | INTEGER  | PRIMARY KEY CHECK (id = 1) | Ensures only a single row exists for the project feature list. |
| `content`    | TEXT     | NOT NULL                   | A JSON string mirroring the `db.FeatureList` struct.           |
| `updated_at` | DATETIME | DEFAULT CURRENT_TIMESTAMP  | Timestamp of the last update.                                  |

#### JSON Structure (`content`)

The `content` column in `project_features` stores a JSON object with the following structure:

- **`project_name`**: `string`
- **`features`**: `Array<Feature>`
  - **`id`**: `string`
  - **`category`**: `string`
  - **`description`**: `string`
  - **`status`**: `string` (e.g., `pending`, `done`, `implemented`)
  - **`passes`**: `boolean`
  - **`steps`**: `Array<string>`
  - **`dependencies`**: `Object`
    - **`depends_on_ids`**: `Array<string>`
    - **`exclusive_write_paths`**: `Array<string>`
    - **`read_only_paths`**: `Array<string>`

---

## Migration and Access

The schema is managed via the `migrate()` method in `internal/db/sqlite.go`. All database interactions should go through the `db.Store` interface to ensure consistency and isolation.
