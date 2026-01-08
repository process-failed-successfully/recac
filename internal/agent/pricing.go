package agent

// ModelPricing defines the cost per million tokens for a given model.
type ModelPricing struct {
	// Name is the identifier for the model (e.g., "gpt-4o").
	Name string
	// PromptPrice is the cost per 1,000,000 prompt (input) tokens in USD.
	PromptPrice float64
	// CompletionPrice is the cost per 1,000,000 completion (output) tokens in USD.
	CompletionPrice float64
}

// pricingTable contains the known pricing for various models.
// Prices are sourced from official provider websites.
var pricingTable = map[string]ModelPricing{
	// OpenAI
	"gpt-4o":         {Name: "gpt-4o", PromptPrice: 5.00, CompletionPrice: 15.00},
	"gpt-4-turbo":    {Name: "gpt-4-turbo", PromptPrice: 10.00, CompletionPrice: 30.00},
	"gpt-3.5-turbo":  {Name: "gpt-3.5-turbo", PromptPrice: 0.50, CompletionPrice: 1.50},

	// Google
	"gemini-1.5-pro": {Name: "gemini-1.5-pro", PromptPrice: 3.50, CompletionPrice: 10.50},
	"gemini-1.5-flash": {Name: "gemini-1.5-flash", PromptPrice: 0.35, CompletionPrice: 1.05},
	"gemini-pro":     {Name: "gemini-pro", PromptPrice: 0.50, CompletionPrice: 1.50},

	// OpenRouter - Note: Prices can be dynamic. These are estimates.
	"openrouter/anthropic/claude-3-opus": {Name: "openrouter/anthropic/claude-3-opus", PromptPrice: 15.00, CompletionPrice: 75.00},

	// Local/Self-Hosted Models (cost is effectively 0)
	"ollama/llama2":   {Name: "ollama/llama2", PromptPrice: 0.0, CompletionPrice: 0.0},
	"ollama/mistral":  {Name: "ollama/mistral", PromptPrice: 0.0, CompletionPrice: 0.0},
	"ollama/codellama": {Name: "ollama/codellama", PromptPrice: 0.0, CompletionPrice: 0.0},
}

// GetPricing returns the pricing information for a given model.
// If the model is not found, it returns a default (zero-cost) pricing struct.
func GetPricing(model string) ModelPricing {
	if price, ok := pricingTable[model]; ok {
		return price
	}
	// Default to zero cost for unknown models
	return ModelPricing{Name: model, PromptPrice: 0.0, CompletionPrice: 0.0}
}

// CalculateCost computes the estimated cost of a session based on token usage and model pricing.
func CalculateCost(model string, promptTokens, completionTokens int) float64 {
	pricing := GetPricing(model)
	promptCost := (float64(promptTokens) / 1_000_000.0) * pricing.PromptPrice
	completionCost := (float64(completionTokens) / 1_000_000.0) * pricing.CompletionPrice
	return promptCost + completionCost
}
