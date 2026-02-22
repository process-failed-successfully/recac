package architecture

import (
	"strings"
	"testing"
)

func TestGenerateMermaid(t *testing.T) {
	tests := []struct {
		name     string
		arch     *SystemArchitecture
		contains []string
	}{
		{
			name: "Basic Components",
			arch: &SystemArchitecture{
				Components: []Component{
					{ID: "api-service", Type: "service"},
					{ID: "user-db", Type: "database"},
				},
			},
			contains: []string{
				"graph TD",
				"api_service[\"api-service\"]",
				"user_db[(\"user-db\")]",
			},
		},
		{
			name: "Relationships",
			arch: &SystemArchitecture{
				Components: []Component{
					{
						ID:   "frontend",
						Type: "frontend",
						Consumes: []Input{
							{Source: "api-service", Type: "HTTP"},
						},
					},
					{
						ID:   "api-service",
						Type: "service",
						Produces: []Output{
							{Target: "queue", Type: "AMQP"},
						},
					},
					{ID: "queue", Type: "queue"},
				},
			},
			contains: []string{
				"frontend((\"frontend\"))",
				"api_service -- \"HTTP\" --> frontend", // Source -- label --> Consumer
				"api_service -- \"AMQP\" --> queue",    // Producer -- label --> Target
				"queue{{\"queue\"}}",
			},
		},
		{
			name: "External Nodes",
			arch: &SystemArchitecture{
				Components: []Component{
					{
						ID:   "worker",
						Type: "worker",
						Consumes: []Input{
							{Source: "external-api", Type: "REST"},
						},
					},
				},
			},
			contains: []string{
				"worker([\"worker\"])",
				"external_api -- \"REST\" --> worker",
				"style external_api stroke-dasharray 5 5",
			},
		},
		{
			name: "Sanitization",
			arch: &SystemArchitecture{
				Components: []Component{
					{ID: "my.service-v1", Type: "service"},
				},
			},
			contains: []string{
				"my_service_v1[\"my.service-v1\"]",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateMermaid(tt.arch)
			for _, substr := range tt.contains {
				if !strings.Contains(got, substr) {
					t.Errorf("GenerateMermaid() missing expected substring: %q\nGot:\n%s", substr, got)
				}
			}
		})
	}
}
