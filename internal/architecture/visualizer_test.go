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
			name: "Empty Architecture",
			arch: &SystemArchitecture{
				Components: []Component{},
			},
			contains: []string{"graph TD"},
		},
		{
			name: "Single Component",
			arch: &SystemArchitecture{
				Components: []Component{
					{ID: "Service A", Type: "service"},
				},
			},
			contains: []string{
				"Service_A[\"Service A\"]",
			},
		},
		{
			name: "Component Types",
			arch: &SystemArchitecture{
				Components: []Component{
					{ID: "DB", Type: "database"},
					{ID: "Queue", Type: "queue"},
					{ID: "Worker", Type: "worker"},
					{ID: "UI", Type: "frontend"},
				},
			},
			contains: []string{
				"DB[(\"DB\")]",
				"Queue{{\"Queue\"}}",
				"Worker([\"Worker\"])",
				"UI((\"UI\"))",
			},
		},
		{
			name: "Consumes Edge",
			arch: &SystemArchitecture{
				Components: []Component{
					{ID: "SvcA", Type: "service"},
					{
						ID:   "SvcB",
						Type: "service",
						Consumes: []Input{
							{Source: "SvcA", Type: "Request"},
						},
					},
				},
			},
			contains: []string{
				"SvcA -->|Request| SvcB",
			},
		},
		{
			name: "Produces Edge",
			arch: &SystemArchitecture{
				Components: []Component{
					{
						ID:   "SvcA",
						Type: "service",
						Produces: []Output{
							{Target: "SvcB", Event: "EventX"},
						},
					},
					{ID: "SvcB", Type: "service"},
				},
			},
			contains: []string{
				"SvcA -->|EventX| SvcB",
			},
		},
		{
			name: "Merged Edges",
			arch: &SystemArchitecture{
				Components: []Component{
					{
						ID:   "SvcA",
						Type: "service",
						Produces: []Output{
							{Target: "SvcB", Event: "Event1"},
							{Target: "SvcB", Event: "Event2"},
						},
					},
					{ID: "SvcB", Type: "service"},
				},
			},
			contains: []string{
				"SvcA -->|Event1, Event2| SvcB",
			},
		},
		{
			name: "Sanitization",
			arch: &SystemArchitecture{
				Components: []Component{
					{ID: "My Service", Type: "service"},
				},
			},
			contains: []string{
				"My_Service[\"My Service\"]",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateMermaid(tt.arch)
			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("GenerateMermaid() = %v, want to contain %v", got, want)
				}
			}
		})
	}
}
