package agent

// PricePerMillionTokens defines the cost in USD per million tokens for a given model.
type PricePerMillionTokens struct {
	Prompt     float64
	Completion float64
}

// PricingTable maps model names to their respective pricing information.
var PricingTable = map[string]PricePerMillionTokens{
	// Google Gemini
	"gemini-1.5-pro-latest":   {Prompt: 7.00, Completion: 21.00},
	"gemini-1.5-flash-latest": {Prompt: 0.70, Completion: 2.10},
	"gemini-pro":              {Prompt: 0.50, Completion: 1.50},

	// OpenAI
	"gpt-4o":        {Prompt: 5.00, Completion: 15.00},
	"gpt-4-turbo":   {Prompt: 10.00, Completion: 30.00},
	"gpt-3.5-turbo": {Prompt: 0.50, Completion: 1.50},

	// Anthropic
	"claude-3-opus-20240229":   {Prompt: 15.00, Completion: 75.00},
	"claude-3-sonnet-20240229": {Prompt: 3.00, Completion: 15.00},
	"claude-3-haiku-20240307":  {Prompt: 0.25, Completion: 1.25},
}

// CalculateCost calculates the estimated cost based on token usage and model pricing.
func CalculateCost(model string, usage TokenUsage) float64 {
	price, ok := PricingTable[model]
	if !ok {
		// Fallback for unknown models
		return float64(usage.TotalTokens) / 1_000_000.0
	}

	promptCost := (float64(usage.TotalPromptTokens) / 1_000_000.0) * price.Prompt
	completionCost := (float64(usage.TotalResponseTokens) / 1_000_000.0) * price.Completion
	return promptCost + completionCost
}
