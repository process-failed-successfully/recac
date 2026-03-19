Let's see: `delay := o.RetryDelay`.
It uses a constant delay. We can add `--retry-backoff` boolean flag.
When `--retry-backoff` is true, the delay doubles for each `RetryCount`.
e.g. `delay = o.RetryDelay * (1 << (job.RetryCount - 1))`

This is a good, solid enhancement!
But maybe we can do something more "bold and innovative".

**Feature: Dynamic Job Workflows / Sub-pipelines.**
Wait, they already have dynamic jobs outputting `RECAC_SPAWN_JOBS`.

What about **Bulk "Mute/Unmute" Notifications for Jobs**?
Sometimes a noisy job spams Slack/Discord. A flag to silence a job.

What about **Job Concurrency Groups Cancellation Policy**?
Currently, if `CancelInProgress` is true, it cancels older ones. What if it's "cancel new ones" or "queue"?

What about **TUI Dashboard feature**? Let's check `internal/tui/dashboard.go`.
