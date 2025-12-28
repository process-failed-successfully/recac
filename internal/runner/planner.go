package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"recac/internal/agent"
	"regexp"
	"strings"
)

type Feature struct {
	Category    string   `json:"category"`
	Description string   `json:"description"`
	Steps       []string `json:"steps"`
	Passes      bool     `json:"passes"`
}

// GenerateFeatureList asks the agent to decompose the spec into features.
func GenerateFeatureList(ctx context.Context, a agent.Agent, spec string) ([]Feature, error) {
	prompt := fmt.Sprintf("You are a Lead Software Architect.\n" +
		"Given the following application specification, decompose it into a list of verifyable features (acceptance tests).\n" +
		"Return ONLY a valid JSON array of objects. Do not include markdown formatting like ```json.\n" +
		"Each object must have:\n" +
		"- \"category\": string (e.g., \"cli\", \"ui\", \"backend\")\n" +
		"- \"description\": string (clear acceptance criteria)\n" +
		"- \"steps\": array of strings (verification steps)\n" +
		"- \"passes\": boolean (always false initially)\n\n" +
		"Specification:\n%s", spec)

	response, err := a.Send(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("agent failed to generate plan: %w", err)
	}

	// Clean response (remove markdown code blocks if present)
	cleanedResponse := cleanJSON(response)

	var features []Feature
	if err := json.Unmarshal([]byte(cleanedResponse), &features); err != nil {
		return nil, fmt.Errorf("failed to parse agent response: %w\nResponse: %s", err, response)
	}

	return features, nil
}

// Pre-compile regexes for performance
var (
	reJSONBlock = regexp.MustCompile("(?s)```json(.*?)```")
	reCodeBlock = regexp.MustCompile("(?s)```(.*?)```")
)

func cleanJSON(input string) string {
	// Remove ```json and ``` lines
	match := reJSONBlock.FindStringSubmatch(input)
	if len(match) > 1 {
		return strings.TrimSpace(match[1])
	}
	// Try without json tag
	match2 := reCodeBlock.FindStringSubmatch(input)
	if len(match2) > 1 {
		return strings.TrimSpace(match2[1])
	}
	return strings.TrimSpace(input)
}
