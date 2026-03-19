Ah, `o.pendingJobs` is just a map, so `GetPendingJobs()` returns them in **random order**!
If a user runs `--list-pending`, they see a random list.
Wait, `o.pendingJobs` is a map!
`evaluatePendingJobs` processes them by sorting them:
```go
	var candidates []*JobInfo
	for _, j := range o.pendingJobs {
		candidates = append(candidates, j)
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].WorkItem.Priority > candidates[j].WorkItem.Priority
	})
```
So internally they are sorted by priority when executing.
BUT `GetPendingJobs()` does NOT sort them! This means the UI, the CLI, and the API return pending jobs in a random order every time.

**Feature Proposal 1: Implement `GET /jobs/queue` or modify `GetPendingJobs` to return pending jobs deterministically sorted by Priority, Delay (RunAfter), and Submit Time (StartTime). This would accurately reflect the orchestrator's execution queue.** This is a bit of "cleanup/documentation" level maybe?

Wait! **Adding the ability to Pause/Resume a specific JOB** (not the whole orchestrator).
If a job is running in Docker, we can `docker pause` and `docker unpause` it. This is extremely useful if a job is doing something heavy and taking all CPU, but you don't want to cancel it, just pause it while you do something else.
Currently there's only `CancelJob` (`DELETE /jobs/{id}`).
We can add:
- `POST /jobs/{id}/pause`
- `POST /jobs/{id}/resume`
And corresponding CLI flags: `--pause-job`, `--resume-job`.
The orchestrator's `spawner_docker.go` and `spawner_process.go` can implement `Pause()` and `Resume()` on the `Spawner` interface.

Let's check if the `Spawner` interface supports Pause/Resume.
