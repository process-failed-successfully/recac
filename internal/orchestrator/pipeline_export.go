package orchestrator

import (
	"gopkg.in/yaml.v3"
)

// ExportPipelineToYAML converts a list of jobs into a Pipeline YAML representation.
// It reconstructs the declarative pipeline from the current state of jobs.
func ExportPipelineToYAML(name string, jobs []JobInfo) ([]byte, error) {
	pipeline := Pipeline{
		Name: name,
		Jobs: make(map[string]PipelineJob),
	}

	for _, job := range jobs {
		timeoutStr := ""
		if job.WorkItem.Timeout > 0 {
			timeoutStr = job.WorkItem.Timeout.String()
		}
		delayStr := ""
		if job.WorkItem.Delay > 0 {
			delayStr = job.WorkItem.Delay.String()
		}
		retryDelayStr := ""
		if job.WorkItem.RetryDelay != nil && *job.WorkItem.RetryDelay > 0 {
			retryDelayStr = job.WorkItem.RetryDelay.String()
		}

		// Ensure we don't save nil slices/maps to keep the YAML clean
		var dependsOn []string
		if len(job.WorkItem.DependsOn) > 0 {
			dependsOn = job.WorkItem.DependsOn
		}

		var tags []string
		if len(job.WorkItem.Tags) > 0 {
			tags = job.WorkItem.Tags
		}

		var envVars map[string]string
		if len(job.WorkItem.EnvVars) > 0 {
			envVars = job.WorkItem.EnvVars
		}

		var cancelInProgress *bool
		if job.WorkItem.CancelInProgress {
			t := true
			cancelInProgress = &t
		}

		var continueOnError *bool
		if job.WorkItem.ContinueOnError {
			t := true
			continueOnError = &t
		}

		// Description is often the task in ad-hoc jobs, while Summary is the summary.
		// For the exported pipeline, we map Description to Task to match declarative format.
		// If both are present, we could do something else, but PipelineJob has both.
		pipeline.Jobs[job.ID] = PipelineJob{
			Summary:          job.WorkItem.Summary,
			Description:      job.WorkItem.Description,
			RepoURL:          job.WorkItem.RepoURL,
			DependsOn:        dependsOn,
			EnvVars:          envVars,
			Tags:             tags,
			Priority:         job.WorkItem.Priority,
			Timeout:          timeoutStr,
			Delay:            delayStr,
			ConcurrencyGroup: job.WorkItem.ConcurrencyGroup,
			CancelInProgress: cancelInProgress,
			AgentProvider:    job.WorkItem.AgentProvider,
			AgentModel:             job.WorkItem.AgentModel,
			MaxRetries:             job.WorkItem.MaxRetries,
			RequireApproval:        job.WorkItem.RequireApproval,
			RetryDelay:             retryDelayStr,
			RetryBackoffMultiplier: job.WorkItem.RetryBackoffMultiplier,
			ContinueOnError:        continueOnError,
		}
	}

	return yaml.Marshal(pipeline)
}
