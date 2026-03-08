import sys

def main():
    with open("cmd/orchestrator/main.go", "r") as f:
        content = f.read()

    # Add flags
    flags_old = """	pflag.Int("max-concurrent-jobs", 0, "Maximum number of concurrent agent jobs allowed (0 = unlimited)")
	pflag.Duration("job-timeout", 0, "Maximum execution time for a job (0 = unlimited)")"""

    flags_new = """	pflag.Int("max-concurrent-jobs", 0, "Maximum number of concurrent agent jobs allowed (0 = unlimited)")
	pflag.Duration("job-timeout", 0, "Maximum execution time for a job (0 = unlimited)")
	pflag.Int("max-retries", 0, "Maximum number of automatic retries for failed jobs")
	pflag.Duration("retry-delay", 5*time.Second, "Delay between automatic retries")"""

    content = content.replace(flags_old, flags_new)

    # Bind flags
    bind_old = """	viper.BindPFlag("orchestrator.max_concurrent_jobs", pflag.Lookup("max-concurrent-jobs"))
	viper.BindPFlag("orchestrator.job_timeout", pflag.Lookup("job-timeout"))"""

    bind_new = """	viper.BindPFlag("orchestrator.max_concurrent_jobs", pflag.Lookup("max-concurrent-jobs"))
	viper.BindPFlag("orchestrator.job_timeout", pflag.Lookup("job-timeout"))
	viper.BindPFlag("orchestrator.max_retries", pflag.Lookup("max-retries"))
	viper.BindPFlag("orchestrator.retry_delay", pflag.Lookup("retry-delay"))"""

    content = content.replace(bind_old, bind_new)

    # Bind Env
    env_old = """	viper.BindEnv("orchestrator.max_concurrent_jobs", "RECAC_MAX_CONCURRENT_JOBS")
	viper.BindEnv("orchestrator.job_timeout", "RECAC_JOB_TIMEOUT")"""

    env_new = """	viper.BindEnv("orchestrator.max_concurrent_jobs", "RECAC_MAX_CONCURRENT_JOBS")
	viper.BindEnv("orchestrator.job_timeout", "RECAC_JOB_TIMEOUT")
	viper.BindEnv("orchestrator.max_retries", "RECAC_MAX_RETRIES")
	viper.BindEnv("orchestrator.retry_delay", "RECAC_RETRY_DELAY")"""

    content = content.replace(env_old, env_new)

    # Set on Orchestrator
    set_old = """	orch.MaxConcurrentJobs = viper.GetInt("orchestrator.max_concurrent_jobs")
	orch.JobTimeout = viper.GetDuration("orchestrator.job_timeout")"""

    set_new = """	orch.MaxConcurrentJobs = viper.GetInt("orchestrator.max_concurrent_jobs")
	orch.JobTimeout = viper.GetDuration("orchestrator.job_timeout")
	orch.MaxRetries = viper.GetInt("orchestrator.max_retries")
	orch.RetryDelay = viper.GetDuration("orchestrator.retry_delay")"""

    content = content.replace(set_old, set_new)

    with open("cmd/orchestrator/main.go", "w") as f:
        f.write(content)

if __name__ == "__main__":
    main()
