# Development Progress

## Completed Features

- **[x] Dynamic Configuration: Set Value**
  - Implemented `recac config set <key> <value>`
  - Verified with integer, boolean, and string values.
  - Verified `config.yaml` updates.

- **[x] Dynamic Configuration: List Keys**
  - Implemented `recac config list-keys`
  - Verified output lists all Viper keys.

- **[x] Dynamic Configuration: List Models**
  - Implemented `recac config list-models`
  - Support for `gemini` and `openrouter` providers.
  - Reads from `gemini-models.json` and `openrouter-models.json`.

- **[x] Jira Workflow: Run with Ticket ID**
  - Implemented `--jira <ID>` flag in `recac start`.
  - Fetches ticket details using `internal/jira`.
  
- **[x] Jira Workflow: Workspace Isolation**
  - Creates timestamped temp workspace `recac-jira-<ID>-<TIMESTAMP>`.
  - Generates `app_spec.txt` from ticket summary and description.

- **[x] Jira Workflow: Status Sync**
  - Automatically transitions ticket to "In Progress" (ID 31) upon session start.
  - Configurable via `jira.transition_id`.

## Pending Features

- [ ] Pre-flight: Git Clean Check
- [ ] Pre-flight: Dependency Auto-Fix
- [ ] Logs: Follow Mode
- [ ] Logs: Smart Filtering
