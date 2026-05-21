package orchestrator

import (
	"context"
	"fmt"

	"recac/internal/utils"
)

const pipelineOptimizePromptTemplate = `You are an expert software architect and DevOps engineer.
Given the following declarative pipeline YAML for our autonomous coding orchestrator, analyze the execution structure and suggest optimizations.
Optimizations could include:
- Parallelizing independent jobs by fixing overly restrictive 'depends_on' arrays.
- Adding appropriate missing dependencies to prevent race conditions.
- Adding sensible timeouts, retries, or tags.

Output ONLY the fully optimized pipeline YAML.
Do NOT wrap your response in markdown code blocks like ` + "```yaml" + `.
Do NOT include any explanations, greetings, or additional text. Just output the pure YAML.

Original pipeline YAML:
%s
`

// OptimizePipelineYAML uses the configured AI agent to analyze a pipeline YAML and return an optimized version.
func OptimizePipelineYAML(ctx context.Context, yamlContent, provider, model, apiKey string) (string, error) {
	aiClient, err := newAgentFunc(provider, apiKey, model, "", "")
	if err != nil {
		return "", fmt.Errorf("failed to initialize AI agent: %w", err)
	}

	fullPrompt := fmt.Sprintf(pipelineOptimizePromptTemplate, yamlContent)

	response, err := aiClient.Send(ctx, fullPrompt)
	if err != nil {
		return "", fmt.Errorf("failed to optimize pipeline with AI: %w", err)
	}

	// Clean up markdown wrapping if the model ignored the instructions
	response = utils.CleanCodeBlock(response)

	return response, nil
}
