package architecture

import (
	"strings"
	"testing"
)

func TestGenerateMermaid(t *testing.T) {
	arch := &SystemArchitecture{
		Components: []Component{
			{
				ID:   "web-server",
				Type: "frontend",
				Consumes: []Input{
					{Source: "db", Type: "query"},
				},
				Produces: []Output{
					{Target: "queue", Event: "job_created"},
				},
			},
			{
				ID:   "db",
				Type: "database",
			},
			{
				ID:   "queue",
				Type: "queue",
			},
			{
				ID:       "worker",
				Type:     "worker",
				Consumes: []Input{{Source: "queue", Type: "job"}},
			},
		},
	}

	got := GenerateMermaid(arch)

	// Check for nodes
	if !strings.Contains(got, "web_server((\"web-server\"))") {
		t.Errorf("Expected web-server node, got:\n%s", got)
	}
	if !strings.Contains(got, "db[(\"db\")]") {
		t.Errorf("Expected db node, got:\n%s", got)
	}
	if !strings.Contains(got, "queue{{\"queue\"}}") {
		t.Errorf("Expected queue node, got:\n%s", got)
	}
	if !strings.Contains(got, "worker([\"worker\"])") {
		t.Errorf("Expected worker node, got:\n%s", got)
	}

	// Check for edges
	if !strings.Contains(got, "db -->|query| web_server") {
		t.Errorf("Expected db->web-server edge, got:\n%s", got)
	}
	if !strings.Contains(got, "web_server -->|job_created| queue") {
		t.Errorf("Expected web-server->queue edge, got:\n%s", got)
	}
	if !strings.Contains(got, "queue -->|job| worker") {
		t.Errorf("Expected queue->worker edge, got:\n%s", got)
	}
}

func TestGenerateMermaid_Sanitization(t *testing.T) {
	arch := &SystemArchitecture{
		Components: []Component{
			{
				ID:   "my service",
				Type: "service",
			},
		},
	}

	got := GenerateMermaid(arch)

	// ID should be "my_service", Label should be "my service"
	// Output: my_service["my service"]
	if !strings.Contains(got, "my_service[\"my service\"]") {
		t.Errorf("Expected sanitized ID and label, got:\n%s", got)
	}
}
