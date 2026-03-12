package orchestrator

import (
	"fmt"
	"sort"
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
		Delay            string `yaml:"delay"`
	} `yaml:"defaults"`
	Jobs map[string]PipelineJob `yaml:"jobs"`
}

type PipelineJob struct {
	Summary          string              `yaml:"summary"`
	Task             string              `yaml:"task"`
	Description      string              `yaml:"description"`
	RepoURL          string              `yaml:"repo_url"`
	DependsOn        []string            `yaml:"depends_on"`
	EnvVars          map[string]string   `yaml:"env_vars"`
	Matrix           map[string][]string `yaml:"matrix"`
	Tags             []string            `yaml:"tags"`
	Priority         int                 `yaml:"priority"`
	Timeout          string              `yaml:"timeout"` // Parse to time.Duration
	Delay            string              `yaml:"delay"`   // Parse to time.Duration
	ConcurrencyGroup string              `yaml:"concurrency_group"`
	CancelInProgress bool                `yaml:"cancel_in_progress"`
	AgentProvider    string              `yaml:"agent_provider"`
	AgentModel       string              `yaml:"agent_model"`
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
	jobGeneratedIDs := make(map[string][]string)

	// Sort jobs to make evaluation deterministic
	var jobKeys []string
	for k := range p.Jobs {
		jobKeys = append(jobKeys, k)
	}
	sort.Strings(jobKeys)

	// Pass 1: Generate all jobs (including matrix permutations)
	for _, jobKey := range jobKeys {
		jobDef := p.Jobs[jobKey]

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
		delayStr := jobDef.Delay
		if delayStr == "" {
			delayStr = p.Defaults.Delay
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

		// Parse delay
		var parsedDelay time.Duration
		if delayStr != "" {
			var err error
			parsedDelay, err = time.ParseDuration(delayStr)
			if err != nil {
				return nil, fmt.Errorf("invalid delay format for job '%s': %w", jobKey, err)
			}
		}

		// Generate combinations deterministically
		var matrixKeys []string
		for k := range jobDef.Matrix {
			matrixKeys = append(matrixKeys, k)
		}
		sort.Strings(matrixKeys)

		var combinations []map[string]string
		var generate func(int, map[string]string)
		generate = func(idx int, current map[string]string) {
			if idx == len(matrixKeys) {
				cp := make(map[string]string)
				for k, v := range current {
					cp[k] = v
				}
				combinations = append(combinations, cp)
				return
			}
			key := matrixKeys[idx]
			for _, val := range jobDef.Matrix[key] {
				current[key] = val
				generate(idx+1, current)
			}
		}
		generate(0, make(map[string]string))

		// If matrix is empty, just generate one combination (empty map)
		if len(combinations) == 0 {
			combinations = append(combinations, make(map[string]string))
		}

		for i, combo := range combinations {
			jobID := fmt.Sprintf("%s-%s-%d", pipelineIDPrefix, sanitizeName(jobKey), timestamp)
			summary := jobDef.Summary

			// If it's a matrix job, append suffix and variables
			suffixParts := []string{}
			for _, k := range matrixKeys {
				if v, ok := combo[k]; ok {
					suffixParts = append(suffixParts, fmt.Sprintf("%s=%s", k, v))
				}
			}

			if len(suffixParts) > 0 {
				jobID = fmt.Sprintf("%s-%d", jobID, i+1)
				summary = fmt.Sprintf("%s [%s]", summary, strings.Join(suffixParts, ", "))
			}

			// Deep copy EnvVars
			envVars := make(map[string]string)
			if jobDef.EnvVars != nil {
				for k, v := range jobDef.EnvVars {
					envVars[k] = v
				}
			}
			for k, v := range combo {
				envVars[k] = v
			}

			// Deep copy Tags
			var tags []string
			if jobDef.Tags != nil {
				tags = make([]string, len(jobDef.Tags))
				copy(tags, jobDef.Tags)
			}

			// Store ID for dependency resolution later
			jobGeneratedIDs[jobKey] = append(jobGeneratedIDs[jobKey], jobID)

			items = append(items, WorkItem{
				ID:               jobID,
				Summary:          summary,
				Description:      description,
				RepoURL:          repoURL,
				EnvVars:          envVars,
				DependsOn:        jobDef.DependsOn, // Store original names for now, resolve in pass 2
				Priority:         jobDef.Priority,
				Tags:             tags,
				Timeout:          parsedTimeout,
				Delay:            parsedDelay,
				ConcurrencyGroup: concurrencyGroup,
				CancelInProgress: jobDef.CancelInProgress,
				AgentProvider:    agentProvider,
				AgentModel:       agentModel,
			})
		}
	}

	// Pass 2: Resolve Dependencies
	for i := range items {
		var resolvedDeps []string
		for _, dep := range items[i].DependsOn {
			if deps, exists := jobGeneratedIDs[dep]; exists {
				resolvedDeps = append(resolvedDeps, deps...)
			} else {
				// We can trace back to original jobKey by parsing ID if we really needed,
				// but easier to just iterate over jobKeys and see if ID matches.
				originalJobKey := ""
				for key, ids := range jobGeneratedIDs {
					for _, id := range ids {
						if id == items[i].ID {
							originalJobKey = key
							break
						}
					}
					if originalJobKey != "" {
						break
					}
				}
				return nil, fmt.Errorf("job '%s' depends on unknown job '%s'", originalJobKey, dep)
			}
		}
		items[i].DependsOn = resolvedDeps
	}

	return items, nil
}
