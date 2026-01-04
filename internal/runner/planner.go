package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"recac/internal/agent"
	"recac/internal/agent/prompts"
	"recac/internal/db"
	"regexp"
	"strings"
)

// GenerateFeatureList asks the agent to decompose the spec into features.
func GenerateFeatureList(ctx context.Context, a agent.Agent, spec string) (*db.FeatureList, error) {
	prompt, err := prompts.GetPrompt(prompts.Planner, map[string]string{
		"spec": spec,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to load planner prompt: %w", err)
	}

	response, err := a.Send(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("agent failed to generate plan: %w", err)
	}

	// Clean response (remove markdown code blocks if present)
	cleanedResponse := cleanJSON(response)

	var featureList db.FeatureList
	if err := json.Unmarshal([]byte(cleanedResponse), &featureList); err != nil {
		return nil, fmt.Errorf("failed to parse agent response: %w\nResponse: %s", err, response)
	}

	return &featureList, nil
}

var (
	reJSONBlock = regexp.MustCompile("(?s)```json(.*?)```")
	reBlock     = regexp.MustCompile("(?s)```(.*?)```")
)

func cleanJSON(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}

	// 1. Try regex for ```json ... ``` (Most explicit)
	match := reJSONBlock.FindStringSubmatch(input)
	if len(match) > 1 {
		return strings.TrimSpace(match[1])
	}

	// 2. Try regex for ``` ... ``` (Any block)
	// We need to be careful not to capture non-JSON blocks if possible, but usually the prompt asks for JSON.
	match2 := reBlock.FindStringSubmatch(input)
	if len(match2) > 1 {
		content := strings.TrimSpace(match2[1])
		// Remove language tag if present in the captured content (regex might have captured it if no newline)
		// actually reBlock is ````(.*?)```, so match[1] includes the tag line if it's there?
		// e.g. ```json\n{...}``` -> match[1] is "json\n{...}"
		// or ```\n{...}``` -> match[1] is "\n{...}"

		if idx := strings.Index(content, "\n"); idx != -1 {
			firstLine := strings.TrimSpace(content[:idx])
			// If first line is short and looks like a tag (no spaces, e.g. "json", "json5"), skip it
			if len(firstLine) < 10 && !strings.Contains(firstLine, " ") && !strings.Contains(firstLine, "{") && !strings.Contains(firstLine, "[") {
				return strings.TrimSpace(content[idx+1:])
			}
		}
		// If no newline, maybe it's just "{...}" or "json{...}" (rare)
		// If it starts with "json" and then immediate brace?
		if strings.HasPrefix(content, "json") {
			return strings.TrimSpace(strings.TrimPrefix(content, "json"))
		}

		return content
	}

	// 3. Fallback: If it looks like a JSON object/array but has text around it (and no backticks)
	// Find first '{' or '[' and last '}' or ']'
	startBrace := strings.Index(input, "{")
	startBracket := strings.Index(input, "[")

	start := -1
	if startBrace != -1 && startBracket != -1 {
		if startBrace < startBracket {
			start = startBrace
		} else {
			start = startBracket
		}
	} else if startBrace != -1 {
		start = startBrace
	} else if startBracket != -1 {
		start = startBracket
	}

	if start != -1 {
		// Find matching end
		endBrace := strings.LastIndex(input, "}")
		endBracket := strings.LastIndex(input, "]")
		end := -1

		if endBrace != -1 && endBracket != -1 {
			if endBrace > endBracket {
				end = endBrace
			} else {
				end = endBracket
			}
		} else if endBrace != -1 {
			end = endBrace
		} else if endBracket != -1 {
			end = endBracket
		}

		if end != -1 && end > start {
			return strings.TrimSpace(input[start : end+1])
		}
	}

	return input
}
