package analysis

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

type CIFinding struct {
	Line     int    `json:"line"`
	Rule     string `json:"rule"`
	Message  string `json:"message"`
	Severity string `json:"severity"` // "info", "warning", "error"
	Advice   string `json:"advice"`
}

// AnalyzeGitHubWorkflow analyzes a GitHub Actions workflow YAML for best practices.
func AnalyzeGitHubWorkflow(content string) ([]CIFinding, error) {
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(content), &node); err != nil {
		return nil, err
	}

	var findings []CIFinding

	// Document node -> Content (Sequence) -> First item is the Map
	if len(node.Content) == 0 {
		return findings, nil
	}

	// The root of the YAML document is usually a Document node containing one Mapping node.
	root := node.Content[0]
	if root.Kind != yaml.MappingNode {
		// Could be a sequence or scalar (invalid workflow), but let's be safe
		return findings, nil
	}

	checkPermissions(root, &findings)
	checkJobs(root, &findings)

	return findings, nil
}

func checkPermissions(root *yaml.Node, findings *[]CIFinding) {
	// Look for "permissions" key in root map
	hasPermissions := false
	for i := 0; i < len(root.Content); i += 2 {
		key := root.Content[i]
		if key.Value == "permissions" {
			hasPermissions = true
			break
		}
	}

	if !hasPermissions {
		*findings = append(*findings, CIFinding{
			Line:     root.Line,
			Rule:     "missing_permissions",
			Message:  "Top-level permissions not defined",
			Severity: "warning",
			Advice:   "Define 'permissions: {}' or specific permissions to follow least privilege.",
		})
	}
}

func checkJobs(root *yaml.Node, findings *[]CIFinding) {
	var jobsNode *yaml.Node
	for i := 0; i < len(root.Content); i += 2 {
		key := root.Content[i]
		if key.Value == "jobs" {
			jobsNode = root.Content[i+1]
			break
		}
	}

	if jobsNode == nil || jobsNode.Kind != yaml.MappingNode {
		return
	}

	// Iterate over jobs
	for i := 0; i < len(jobsNode.Content); i += 2 {
		// jobKey := jobsNode.Content[i]
		jobValue := jobsNode.Content[i+1]

		checkJobTimeout(jobValue, findings)
		checkJobSteps(jobValue, findings)
	}
}

func checkJobTimeout(jobNode *yaml.Node, findings *[]CIFinding) {
	hasTimeout := false
	for i := 0; i < len(jobNode.Content); i += 2 {
		key := jobNode.Content[i]
		if key.Value == "timeout-minutes" {
			hasTimeout = true
			break
		}
	}

	if !hasTimeout {
		*findings = append(*findings, CIFinding{
			Line:     jobNode.Line,
			Rule:     "missing_timeout",
			Message:  "Job is missing 'timeout-minutes'",
			Severity: "warning",
			Advice:   "Set a timeout to prevent stalled jobs from consuming minutes (default is 6h).",
		})
	}
}

func checkJobSteps(jobNode *yaml.Node, findings *[]CIFinding) {
	var stepsNode *yaml.Node
	for i := 0; i < len(jobNode.Content); i += 2 {
		key := jobNode.Content[i]
		if key.Value == "steps" {
			stepsNode = jobNode.Content[i+1]
			break
		}
	}

	if stepsNode == nil || stepsNode.Kind != yaml.SequenceNode {
		return
	}

	for _, step := range stepsNode.Content {
		checkStepPinning(step, findings)
		checkStepCache(step, findings)
	}
}

func checkStepPinning(step *yaml.Node, findings *[]CIFinding) {
	for i := 0; i < len(step.Content); i += 2 {
		key := step.Content[i]
		if key.Value == "uses" {
			val := step.Content[i+1]
			action := val.Value

			// Ignore local actions
			if strings.HasPrefix(action, "./") {
				continue
			}

			// Check for mutable tags (@latest, @master, @main, @v1, etc.)
			parts := strings.Split(action, "@")
			if len(parts) > 1 {
				ref := parts[1]
				isSHA := len(ref) >= 40 // Simple heuristic for SHA

				if ref == "latest" || ref == "master" || ref == "main" || strings.HasPrefix(ref, "v") {
					severity := "info"
					rule := "action_ref_tag"
					msg := "Action uses version tag instead of SHA"

					if ref == "latest" || ref == "master" || ref == "main" {
						severity = "warning"
						rule = "unpinned_action"
						msg = fmt.Sprintf("Action uses mutable '%s' ref", ref)
					}

					*findings = append(*findings, CIFinding{
						Line:     val.Line,
						Rule:     rule,
						Message:  msg,
						Severity: severity,
						Advice:   "Pin to a specific SHA (immutable) for supply chain security.",
					})
				} else if !isSHA {
					// Likely a branch or short tag
					*findings = append(*findings, CIFinding{
						Line:     val.Line,
						Rule:     "unpinned_action",
						Message:  fmt.Sprintf("Action uses likely mutable ref '%s'", ref),
						Severity: "warning",
						Advice:   "Pin to a specific SHA (immutable) for supply chain security.",
					})
				}
			}
		}
	}
}

func checkStepCache(step *yaml.Node, findings *[]CIFinding) {
	// Heuristic: check if uses setup-go/setup-node/setup-python/setup-java
	// AND check if 'with' block has 'cache' key.
	// Or newer versions handle caching differently.

	usesSetup := false
	var withNode *yaml.Node

	for i := 0; i < len(step.Content); i += 2 {
		key := step.Content[i]
		if key.Value == "uses" {
			val := step.Content[i+1].Value
			if strings.Contains(val, "actions/setup-go") ||
			   strings.Contains(val, "actions/setup-node") ||
			   strings.Contains(val, "actions/setup-python") ||
			   strings.Contains(val, "actions/setup-java") {
				usesSetup = true
			}
		}
		if key.Value == "with" {
			withNode = step.Content[i+1]
		}
	}

	if usesSetup {
		hasCache := false
		if withNode != nil {
			for i := 0; i < len(withNode.Content); i += 2 {
				key := withNode.Content[i]
				if key.Value == "cache" {
					hasCache = true
					break
				}
			}
		}

		if !hasCache {
			*findings = append(*findings, CIFinding{
				Line:     step.Line,
				Rule:     "missing_cache",
				Message:  "Setup action missing built-in caching",
				Severity: "info",
				Advice:   "Enable caching (e.g., 'cache: true' or 'cache: gomod') to speed up builds.",
			})
		}
	}
}
