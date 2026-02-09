package scenarios

import (
	"strings"
	"testing"
)

type MetadataProvider interface {
	Name() string
	Description() string
	AppSpec(repoURL string) string
	Generate(uniqueID string, repoURL string) []TicketSpec
}

func TestScenarioMetadata(t *testing.T) {
	scenarios := []struct {
		scenario MetadataProvider
		id       string
	}{
		{&DistributedLogScenario{}, "LOG"},
		{&LoadBalancerScenario{}, "LB"},
		{&PrimePythonScenario{}, "PRIMES"},
		{&HTTPProxyScenario{}, "PROXY"},
		{&RedisChallengeScenario{}, "REDIS"},
		{&SQLParserScenario{}, "SQL-PARSER"},
	}

	for _, tc := range scenarios {
		t.Run(tc.scenario.Name(), func(t *testing.T) {
			// Test Name
			if tc.scenario.Name() == "" {
				t.Error("Name() should not be empty")
			}

			// Test Description
			if tc.scenario.Description() == "" {
				t.Error("Description() should not be empty")
			}

			// Test AppSpec
			appSpec := tc.scenario.AppSpec("http://example.com/repo")
			if appSpec == "" {
				t.Error("AppSpec() should not return empty string")
			}
			if !strings.Contains(appSpec, "http://example.com/repo") {
				t.Error("AppSpec() should contain the repo URL")
			}

			// Test Generate
			specs := tc.scenario.Generate("test-id", "http://example.com/repo")
			if len(specs) == 0 {
				t.Error("Generate() should return at least one ticket spec")
			}

			for _, spec := range specs {
				if spec.ID == "" {
					t.Error("Ticket ID should not be empty")
				}
				if spec.Type == "" {
					t.Error("Ticket Type should not be empty")
				}
				if spec.Summary == "" {
					t.Error("Ticket Summary should not be empty")
				}
				if spec.Desc == "" {
					t.Error("Ticket Description should not be empty")
				}
			}
		})
	}
}
