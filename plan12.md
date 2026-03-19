So `completedJobs` is just an array slice in memory. If 1,000,000 jobs complete, the RAM usage explodes.
Ah! A **Retention Policy** or **Auto-Purge for Completed Jobs**.
We can add `--history-limit` or `--retain-jobs` parameter, which automatically purges older jobs from memory and the DB when the limit is exceeded.
Or a `MaxCompletedJobs` in `config` that defaults to say `1000`.

Wait, let's see if there's already a `MaxCompletedJobs` setting.
