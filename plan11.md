Okay, `importJobs` exists.
Let's consider **Bulk Bulk Updates**: We have bulk update tags, dependencies, timeout, priority, environment variables. We have bulk force-complete, clone, cancel, delete, purge, approve, hold, unhold, skip, heal, archive, export.

What about **Bulk "Assign Agent"**? wait, `--update-agent-match` exists.

What is a *really useful* but missing feature?
**Job Output Dependencies (Dynamic Output Parsing)**.
Wait! The memory mentions: "The orchestrator supports dynamic job generation during runtime. A successfully completed job can automatically spawn new downstream jobs by outputting a JSON array of `WorkItem` definitions assigned to the specific output variable key `RECAC_SPAWN_JOBS`." This already exists!

What about **Matrix Pipelines with conditions**?
"The orchestrator supports conditional job execution based on dependency outcomes using the run_condition property (always, on_failure, on_success)". This exists too.

What about **Pipelines from Git repository**? E.g., `--submit-git-pipeline "https://github.com/.../pipeline.yaml"`. So the orchestrator fetches the pipeline definition dynamically from a Git repo.
Wait, that's slightly complex.

What about **Bulk "Retry" from a specific point in the DAG**?
There's `POST /jobs/{id}/retry?downstream=true`.

Let's check the **Janitor functionality**. `Janitor` cleans up old Docker containers and log files. Does it clean up the SQLite database or `completedJobs` slice?
If you have 10,000 completed jobs, the `completedJobs` slice in memory will grow forever!
Wait! Does `completedJobs` ever get purged automatically?
Let's check `orchestrator.go`.
