package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEvaluateIfCondition(t *testing.T) {
	env := map[string]string{
		"TRUE_VAR":       "true",
		"FALSE_VAR":      "false",
		"NUMBER_VAR":     "1",
		"ZERO_VAR":       "0",
		"STRING_VAR":     "hello",
		"OTHER_STR_VAR":  "world",
		"MATCH_STR_VAR":  "hello",
		"EMPTY_VAR":      "",
	}

	tests := []struct {
		name      string
		condition string
		expected  bool
	}{
		// Boolean evaluation
		{"Empty condition", "", true},
		{"True boolean literal", "true", true},
		{"False boolean literal", "false", false},
		{"True variable", "${TRUE_VAR}", true},
		{"False variable", "${FALSE_VAR}", false},
		{"Number 1 variable", "${NUMBER_VAR}", true},
		{"Number 0 variable", "${ZERO_VAR}", false},
		{"Empty variable", "${EMPTY_VAR}", false},
		{"Missing variable", "${MISSING_VAR}", false},

		// NOT boolean evaluation
		{"Not true", "!true", false},
		{"Not false", "!false", true},
		{"Not true variable", "!${TRUE_VAR}", false},
		{"Not false variable", "!${FALSE_VAR}", true},

		// Equality
		{"String equality true", "'hello' == 'hello'", true},
		{"String equality false", "'hello' == 'world'", false},
		{"Variable equality true", "'${STRING_VAR}' == 'hello'", true},
		{"Variable equality false", "'${STRING_VAR}' == 'world'", false},
		{"Variable to variable equality true", "'${STRING_VAR}' == '${MATCH_STR_VAR}'", true},
		{"Variable to variable equality false", "'${STRING_VAR}' == '${OTHER_STR_VAR}'", false},

		// Inequality
		{"String inequality true", "'hello' != 'world'", true},
		{"String inequality false", "'hello' != 'hello'", false},
		{"Variable inequality true", "'${STRING_VAR}' != 'world'", true},
		{"Variable inequality false", "'${STRING_VAR}' != 'hello'", false},

		// Numeric comparisons
		{"Numeric greater than true", "10 > 5", true},
		{"Numeric greater than false", "5 > 10", false},
		{"Numeric greater than equal true", "10 >= 10", true},
		{"Numeric greater than equal false", "5 >= 10", false},
		{"Numeric less than true", "5 < 10", true},
		{"Numeric less than false", "10 < 5", false},
		{"Numeric less than equal true", "10 <= 10", true},
		{"Numeric less than equal false", "10 <= 5", false},
		{"Numeric with float true", "10.5 > 10.4", true},
		{"Numeric with variable true", "${NUMBER_VAR} < 5", true},
		{"Numeric with variable false", "${NUMBER_VAR} >= 5", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := EvaluateIfCondition(tt.condition, env)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
