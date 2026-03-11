package orchestrator

import (
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Pipeline struct {
	Name     string `yaml:"name"`
	Defaults struct {
		RepoURL          string `yaml:"repo_url"`
		AgentProvider    string `yaml:"agent_provider"`
		AgentModel       string `yaml:"agent_model"`
		ConcurrencyGroup string `yaml:"concurrency_group"`
	} `yaml:"defaults"`
	Jobs map[string]PipelineJob `yaml:"jobs"`
}

type PipelineJob struct {
	Summary          string            `yaml:"summary"`
	Task             string            `yaml:"task"`
	Description      string            `yaml:"description"`
	RepoURL          string            `yaml:"repo_url"`
	DependsOn        []string          `yaml:"depends_on"`
	EnvVars          map[string]string `yaml:"env_vars"`
	Tags             []string          `yaml:"tags"`
	Priority         int               `yaml:"priority"`
	Timeout          string            `yaml:"timeout"` // Parse to time.Duration
	ConcurrencyGroup string            `yaml:"concurrency_group"`
	CancelInProgress bool              `yaml:"cancel_in_progress"`
	AgentProvider    string            `yaml:"agent_provider"`
	AgentModel       string            `yaml:"agent_model"`
}

// sanitizeName creates a safe string for IDs
func sanitizeName(name string) string {
	s := strings.ToLower(name)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	return s
}

// ParsePipelineToWorkItems converts a YAML pipeline definition into a list of WorkItems
func ParsePipelineToWorkItems(yamlData []byte) ([]WorkItem, error) {
	var p Pipeline
	if err := yaml.Unmarshal(yamlData, &p); err != nil {
		return nil, fmt.Errorf("failed to unmarshal pipeline YAML: %w", err)
	}

	if p.Name == "" {
		return nil, fmt.Errorf("pipeline must have a name")
	}
	if len(p.Jobs) == 0 {
		return nil, fmt.Errorf("pipeline must have at least one job")
	}

	pipelineIDPrefix := sanitizeName(p.Name)
	timestamp := time.Now().UnixNano()

	var items []WorkItem

	for jobKey, jobDef := range p.Jobs {
		jobID := fmt.Sprintf("%s-%s-%d", pipelineIDPrefix, sanitizeName(jobKey), timestamp)

		// Resolve dependencies. We map 'depends_on: [build]' to 'pipeline-name-build-<timestamp>'
		var resolvedDeps []string
		for _, dep := range jobDef.DependsOn {
			if _, exists := p.Jobs[dep]; !exists {
				return nil, fmt.Errorf("job '%s' depends on unknown job '%s'", jobKey, dep)
			}
			resolvedDeps = append(resolvedDeps, fmt.Sprintf("%s-%s-%d", pipelineIDPrefix, sanitizeName(dep), timestamp))
		}

		// Apply defaults
		repoURL := jobDef.RepoURL
		if repoURL == "" {
			repoURL = p.Defaults.RepoURL
		}
		agentProvider := jobDef.AgentProvider
		if agentProvider == "" {
			agentProvider = p.Defaults.AgentProvider
		}
		agentModel := jobDef.AgentModel
		if agentModel == "" {
			agentModel = p.Defaults.AgentModel
		}
		concurrencyGroup := jobDef.ConcurrencyGroup
		if concurrencyGroup == "" {
			concurrencyGroup = p.Defaults.ConcurrencyGroup
		}

		// Use Task as Description if Description is empty
		description := jobDef.Description
		if description == "" {
			description = jobDef.Task
		}

		// Parse timeout
		var parsedTimeout time.Duration
		if jobDef.Timeout != "" {
			var err error
			parsedTimeout, err = time.ParseDuration(jobDef.Timeout)
			if err != nil {
				return nil, fmt.Errorf("invalid timeout format for job '%s': %w", jobKey, err)
			}
		}

		// EnvVars copy
		var envVars map[string]string
		if jobDef.EnvVars != nil {
			envVars = make(map[string]string)
			for k, v := range jobDef.EnvVars {
				envVars[k] = v
			}
		}

		// Tags copy
		var tags []string
		if jobDef.Tags != nil {
			tags = make([]string, len(jobDef.Tags))
			copy(tags, jobDef.Tags)
		}

		items = append(items, WorkItem{
			ID:               jobID,
			Summary:          jobDef.Summary,
			Description:      description,
			RepoURL:          repoURL,
			EnvVars:          envVars,
			DependsOn:        resolvedDeps,
			Priority:         jobDef.Priority,
			Tags:             tags,
			Timeout:          parsedTimeout,
			ConcurrencyGroup: concurrencyGroup,
			CancelInProgress: jobDef.CancelInProgress,
			AgentProvider:    agentProvider,
			AgentModel:       agentModel,
		})
	}

	return items, nil
}
