package architecture

import (
	"strings"
	"testing"
)

func TestGenerateMermaid(t *testing.T) {
	tests := []struct {
		name     string
		arch     SystemArchitecture
		contains []string
	}{
		{
			name: "Empty",
			arch: SystemArchitecture{
				SystemName: "EmptySys",
			},
			contains: []string{"flowchart TD"},
		},
		{
			name: "Single Component",
			arch: SystemArchitecture{
				SystemName: "SingleSys",
				Components: []Component{
					{ID: "comp1", Type: "service"},
				},
			},
			contains: []string{"comp1[\"<b>comp1</b><br/><i>service</i>\"]"},
		},
		{
			name: "Consumes Relationship",
			arch: SystemArchitecture{
				SystemName: "ConsumesSys",
				Components: []Component{
					{ID: "comp1", Type: "service"},
					{ID: "comp2", Type: "worker", Consumes: []Input{{Source: "comp1", Type: "EventA"}}},
				},
			},
			contains: []string{
				"comp1 -->|EventA| comp2",
				"comp1[\"<b>comp1</b><br/><i>service</i>\"]",
				"comp2[\"<b>comp2</b><br/><i>worker</i>\"]",
			},
		},
		{
			name: "Produces Relationship",
			arch: SystemArchitecture{
				SystemName: "ProducesSys",
				Components: []Component{
					{ID: "comp1", Type: "service", Produces: []Output{{Target: "comp2", Event: "EventB"}}},
					{ID: "comp2", Type: "worker"},
				},
			},
			contains: []string{
				"comp1 -->|EventB| comp2",
				"comp1[\"<b>comp1</b><br/><i>service</i>\"]",
				"comp2[\"<b>comp2</b><br/><i>worker</i>\"]",
			},
		},
		{
			name: "Produces with Type fallback",
			arch: SystemArchitecture{
				SystemName: "ProducesSys",
				Components: []Component{
					{ID: "comp1", Type: "service", Produces: []Output{{Target: "comp2", Type: "DataC"}}},
					{ID: "comp2", Type: "worker"},
				},
			},
			contains: []string{
				"comp1 -->|DataC| comp2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateMermaid(&tt.arch)
			for _, want := range tt.contains {
				if !strings.Contains(got, want) {
					t.Errorf("GenerateMermaid() = %v, want to contain %v", got, want)
				}
			}
		})
	}
}
