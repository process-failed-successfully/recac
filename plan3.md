Wait, if `--wait-job` currently streams logs while waiting, what is it using?
Let's look at `waitForJob`:
It polls `/jobs/%s` and if status is "Spawning"|"Running"|"Active", it calls `/jobs/%s/logs` and streams it.
But since `/jobs/%s/logs` just calls `GetLogs()`, and `GetLogs` on Docker `ContainerLogs` uses `ShowStdout: true, ShowStderr: true`, it might just be dumping the logs that exist so far and closing the stream.
Ah! `ContainerLogs` does NOT set `Follow: true`. So it's not actually streaming live logs in a long-lived HTTP connection, it just returns the current logs!
Wait, let's check `internal/docker/client.go:427`:
