Let's consider **Web UI**!
`internal/orchestrator/webui.go` serves `GET /{$}`. So it has a basic web interface.
Can we add a really cool feature to the Web UI? Like a "Restart Job" button, or "Pause/Resume Orchestrator" button, or "Real-time log viewer modal"?
Wait, it might already have them.
Let's look at `webui.go`
