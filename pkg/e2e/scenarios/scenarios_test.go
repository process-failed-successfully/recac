package scenarios

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScenarios(t *testing.T) {
	// The Registry is populated by init() functions.
	// We can iterate over all registered scenarios and test common properties.

	if len(Registry) == 0 {
		t.Skip("No scenarios registered")
	}

	for name, scenario := range Registry {
		t.Run(name, func(t *testing.T) {
			// 1. Name
			assert.NotEmpty(t, scenario.Name())
			assert.Equal(t, name, scenario.Name())

			// 2. Description
			assert.NotEmpty(t, scenario.Description())

			// 3. AppSpec
			appSpec := scenario.AppSpec("http://example.com/repo")
			assert.NotEmpty(t, appSpec)
			assert.Contains(t, appSpec, "http://example.com/repo")

			// 4. Generate
			uniqueID := "TEST-ID"
			tickets := scenario.Generate(uniqueID, "http://example.com/repo")
			assert.NotEmpty(t, tickets)

			for _, ticket := range tickets {
				assert.NotEmpty(t, ticket.ID)
				// Summary usually contains the unique ID
				if strings.Contains(ticket.Summary, "%s") {
					// If it contains %s, it means formatting failed or logic is weird, but here we expect formatted string
					t.Errorf("Summary seems unformatted: %s", ticket.Summary)
				}
				assert.NotEmpty(t, ticket.Desc)
				// Desc should contain repo URL
				assert.Contains(t, ticket.Desc, "http://example.com/repo")
			}
		})
	}
}

// Additional specific tests if needed for complex logic in Generate
func TestDistributedLogScenario_Generate(t *testing.T) {
	s := &DistributedLogScenario{}
	tickets := s.Generate("123", "url")
	assert.Len(t, tickets, 1)
	assert.Equal(t, "LOG", tickets[0].ID)
}

func TestLoadBalancerScenario_Generate(t *testing.T) {
	s := &LoadBalancerScenario{}
	tickets := s.Generate("123", "url")
	assert.Len(t, tickets, 1)
	assert.Equal(t, "LB", tickets[0].ID)
}

func TestPrimePythonScenario_Generate(t *testing.T) {
	s := &PrimePythonScenario{}
	tickets := s.Generate("123", "url")
	assert.Len(t, tickets, 1)
	assert.Equal(t, "PRIMES", tickets[0].ID)
}

func TestHTTPProxyScenario_Generate(t *testing.T) {
	s := &HTTPProxyScenario{}
	tickets := s.Generate("123", "url")
	assert.NotEmpty(t, tickets)
	// It has many tickets
	assert.Equal(t, "INIT", tickets[0].ID)
}

func TestRedisChallengeScenario_Generate(t *testing.T) {
	s := &RedisChallengeScenario{}
	tickets := s.Generate("123", "url")
	assert.Len(t, tickets, 1)
	assert.Equal(t, "REDIS", tickets[0].ID)
}

func TestSQLParserScenario_Generate(t *testing.T) {
	s := &SQLParserScenario{}
	tickets := s.Generate("123", "url")
	assert.Len(t, tickets, 1)
	assert.Equal(t, "SQL-PARSER", tickets[0].ID)
}
