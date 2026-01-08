package agent

import "fmt"

// ModelPricing defines the cost per million tokens for a given model.
type ModelPricing struct {
	PromptCost     float64 // Cost per 1,000,000 prompt tokens
	CompletionCost float64 // Cost per 1,000,000 completion tokens
}

// PricingTable maps model names to their pricing information.
var PricingTable = map[string]ModelPricing{
	// OpenAI
	"gpt-4-turbo":       {PromptCost: 10.00, CompletionCost: 30.00},
	"gpt-4":             {PromptCost: 30.00, CompletionCost: 60.00},
	"gpt-3.5-turbo":     {PromptCost: 0.50, CompletionCost: 1.50},
	"gpt-3.5-turbo-16k": {PromptCost: 3.00, CompletionCost: 4.00},

	// Google
	"gemini-pro": {PromptCost: 1.00, CompletionCost: 2.00},

	// OpenRouter
	"openrouter/auto": {PromptCost: 1.00, CompletionCost: 1.00}, // Default

	// Add other models here
}

// CalculateCost calculates the estimated cost of a session based on token usage and model.
func CalculateCost(model string, promptTokens, completionTokens int) (float64, error) {
	pricing, ok := PricingTable[model]
	if !ok {
		// Fallback to a default if model not in table
		// This prevents errors for custom/Ollama models.
		pricing, ok = PricingTable["openrouter/auto"]
		if !ok {
			return 0, fmt.Errorf("default pricing not found")
		}
	}

	promptCost := (float64(promptTokens) / 1_000_000.0) * pricing.PromptCost
	completionCost := (float64(completionTokens) / 1_000_000.0) * pricing.CompletionCost

	return promptCost + completionCost, nil
}
