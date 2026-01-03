# RECAC Metrics Reference

This document lists all the Prometheus metrics exposed by the RECAC application. These metrics are available at the `/metrics` endpoint (default port: 2112).

## 1. Code Generation

| Metric Name                   | Type    | Description                            |
| :---------------------------- | :------ | :------------------------------------- |
| `recac_lines_generated_total` | Counter | Total lines of code written by agents. |
| `recac_files_created_total`   | Counter | Total new files created.               |
| `recac_files_modified_total`  | Counter | Total files modified.                  |
| `recac_build_success_total`   | Counter | Number of successful builds.           |
| `recac_build_failure_total`   | Counter | Number of failed builds.               |

## 2. Agent Performance

| Metric Name                         | Type      | Description                                      |
| :---------------------------------- | :-------- | :----------------------------------------------- |
| `recac_agent_iterations_total`      | Counter   | Total agent turns/iterations.                    |
| `recac_agent_response_time_seconds` | Histogram | Latency of LLM responses.                        |
| `recac_token_usage_total`           | Counter   | Total tokens used (prompt + completion).         |
| `recac_agent_stalls_total`          | Counter   | Number of times agents stalled/made no progress. |
| `recac_context_window_usage`        | Gauge     | Current percentage of context window usage.      |

## 3. Multi-Agent Orchestration

| Metric Name                      | Type    | Description                                           |
| :------------------------------- | :------ | :---------------------------------------------------- |
| `recac_active_agents`            | Gauge   | Number of currently running agent workers.            |
| `recac_tasks_pending`            | Gauge   | Number of tasks in the queue.                         |
| `recac_tasks_completed_total`    | Counter | Total completed tasks.                                |
| `recac_lock_contention_total`    | Counter | Number of times agents failed to acquire a file lock. |
| `recac_orchestrator_loops_total` | Counter | Number of scheduling cycles.                          |

## 4. System Reliability

| Metric Name                 | Type       | Description                      |
| :-------------------------- | :--------- | :------------------------------- |
| `recac_errors_total`        | CounterVec | Total internal errors by type.   |
| `recac_db_operations_total` | Counter    | Total database reads/writes.     |
| `recac_docker_ops_total`    | Counter    | Total Docker command executions. |
| `recac_docker_errors_total` | Counter    | Docker execution failures.       |
| `recac_uptime_seconds`      | Gauge      | Session duration in seconds.     |
