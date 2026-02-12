package billing

import (
	"errors"
	"fmt"
	"recac/internal/runner"

	"github.com/spf13/viper"
)

var ErrBudgetNotSet = errors.New("budget not set")

// SetBudget sets the budget limit in the configuration.
func SetBudget(amount float64) error {
	viper.Set("budget.limit", amount)
	return viper.WriteConfig()
}

// GetBudget retrieves the budget limit from the configuration.
func GetBudget() float64 {
	return viper.GetFloat64("budget.limit")
}

// CheckBudget calculates current usage and compares it against the budget.
// Returns usage, limit, remaining, and error.
func CheckBudget(sessions []*runner.SessionState) (float64, float64, float64, error) {
	limit := GetBudget()
	if limit == 0 {
		return 0, 0, 0, ErrBudgetNotSet
	}

	analysis, err := AnalyzeSessionCosts(sessions, 0) // No limit on sessions for total calculation
	if err != nil {
		return 0, 0, 0, fmt.Errorf("failed to analyze session costs: %w", err)
	}

	usage := analysis.TotalCost
	remaining := limit - usage
	if remaining < 0 {
		remaining = 0
	}

	return usage, limit, remaining, nil
}
