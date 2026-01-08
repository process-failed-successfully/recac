package agent

// PricingInfo holds the cost per million tokens for a given model.
type PricingInfo struct {
	PromptCost     float64
	CompletionCost float64
}

// pricingTable stores the pricing information for various models.
var pricingTable = map[string]PricingInfo{
	"default":              {PromptCost: 1.00, CompletionCost: 1.00}, // A default fallback
	"gemini-1.5-flash":     {PromptCost: 0.70, CompletionCost: 2.10},
	"gemini-1.5-pro":       {PromptCost: 7.00, CompletionCost: 21.00},
	"gemini-1.0-pro":       {PromptCost: 0.50, CompletionCost: 1.50},
	"gpt-4o":               {PromptCost: 5.00, CompletionCost: 15.00},
	"gpt-4-turbo":          {PromptCost: 10.00, CompletionCost: 30.00},
	"gpt-3.5-turbo":        {PromptCost: 0.50, CompletionCost: 1.50},
	"claude-3-opus":        {PromptCost: 15.00, CompletionCost: 75.00},
	"claude-3-sonnet":      {PromptCost: 3.00, CompletionCost: 15.00},
	"claude-3-haiku":       {PromptCost: 0.25, CompletionCost: 1.25},
	"mistral-large-latest": {PromptCost: 8.00, CompletionCost: 24.00},
	"mistral-small-latest": {PromptCost: 2.00, CompletionCost: 6.00},
}

// GetPricing returns the pricing information for a given model.
// If the model is not found, it returns the default pricing.
func GetPricing(model string) (PricingInfo, bool) {
	price, ok := pricingTable[model]
	if !ok {
		// If the model is not in the table, return the default pricing
		return pricingTable["default"], false
	}
	return price, true
}
