package orchestrator

import (
	"context"
	"fmt"

	"recac/internal/utils"
)

const pipelineGeneratePromptTemplate = `You are an expert software architect and DevOps engineer.
Given the following user request, design a declarative job pipeline for our autonomous coding orchestrator.
The pipeline format MUST be strictly valid YAML.
Do NOT wrap your response in markdown code blocks like ` + "```yaml" + `.
Do NOT include any explanations, greetings, or additional text. Just output the pure YAML.

Example structure:
name: <pipeline-name>
defaults:
  repo_url: <optional-default-repo>
jobs:
  job-1:
    summary: <short summary of the job>
    task: <detailed instructions for the agent>
    tags: [backend, setup]
  job-2:
    summary: <short summary>
    task: <detailed instructions>
    depends_on: [job-1]

User request: %s
`

// GeneratePipelineYAML uses the configured AI agent to generate a pipeline YAML based on a user prompt.
func GeneratePipelineYAML(ctx context.Context, prompt, provider, model, apiKey string) (string, error) {
	aiClient, err := newAgentFunc(provider, apiKey, model, "", "")
	if err != nil {
		return "", fmt.Errorf("failed to initialize AI agent: %w", err)
	}

	fullPrompt := fmt.Sprintf(pipelineGeneratePromptTemplate, prompt)

	response, err := aiClient.Send(ctx, fullPrompt)
	if err != nil {
		return "", fmt.Errorf("failed to generate pipeline from AI: %w", err)
	}

	// Clean up markdown wrapping if the model ignored the instructions
	response = utils.CleanCodeBlock(response)

	return response, nil
}
