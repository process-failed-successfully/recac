package architecture

import (
	"strings"
	"testing"
)

func TestGenerateMermaid(t *testing.T) {
	arch := &SystemArchitecture{
		Components: []Component{
			{
				ID:          "web-app",
				Type:        "frontend",
				Description: "Web Frontend",
				Consumes: []Input{
					{Source: "api-service", Type: "HTTP"},
				},
			},
			{
				ID:          "api-service",
				Type:        "service",
				Description: "Main API",
				Consumes: []Input{
					{Source: "db-main"},
				},
				Produces: []Output{
					{Target: "audit-queue", Type: "Event"},
				},
			},
			{
				ID:          "db-main",
				Type:        "database",
				Description: "Primary DB",
				Consumes: []Input{
					// Duplicate input type should be merged or ignored
					{Source: "api-service", Type: "HTTP"}, // Circular ref just for testing
					{Source: "api-service", Type: "HTTP"},
				},
			},
			{
				ID:          "audit-queue",
				Type:        "queue",
				Description: "Audit Logs",
			},
		},
	}

	mermaid := GenerateMermaid(arch)

	// Check for component definitions
	// Note: Strings might vary slightly depending on spacing in GenerateMermaid, so we check key parts.

	// frontend: web_app
	if !strings.Contains(mermaid, "web_app[/\"web-app\\n(frontend)\"/]:::frontend") {
		t.Errorf("Expected frontend node definition, got:\n%s", mermaid)
	}
	// service: api_service
	if !strings.Contains(mermaid, "api_service(\"api-service\\n(service)\"):::service") {
		t.Errorf("Expected service node definition, got:\n%s", mermaid)
	}
	// database: db_main
	if !strings.Contains(mermaid, "db_main[(\"db-main\\n(database)\")]:::database") {
		t.Errorf("Expected database node definition, got:\n%s", mermaid)
	}
	// queue: audit_queue
	if !strings.Contains(mermaid, "audit_queue{{\"audit-queue\\n(queue)\"}}:::queue") {
		t.Errorf("Expected queue node definition, got:\n%s", mermaid)
	}

	// Check for edges
	// api_service -> web_app (HTTP)
	if !strings.Contains(mermaid, "api_service -->|HTTP| web_app") {
		t.Errorf("Expected edge api_service -> web_app, got:\n%s", mermaid)
	}
	// db_main -> api_service (no label)
	// Note: Depending on implementation, it might be just "-->"
	if !strings.Contains(mermaid, "db_main --> api_service") && !strings.Contains(mermaid, "db_main -->|database| api_service") {
		// My implementation uses empty string if no type provided.
		t.Errorf("Expected edge db_main -> api_service, got:\n%s", mermaid)
	}
	// api_service -> audit_queue (Event)
	if !strings.Contains(mermaid, "api_service -->|Event| audit_queue") {
		t.Errorf("Expected edge api_service -> audit_queue, got:\n%s", mermaid)
	}

	// api_service -> db_main (HTTP) - Check for duplicate removal
	// Should be "|HTTP|", not "|HTTP, HTTP|"
	if !strings.Contains(mermaid, "api_service -->|HTTP| db_main") {
		t.Errorf("Expected edge api_service -> db_main with single label, got:\n%s", mermaid)
	}
	if strings.Contains(mermaid, "|HTTP, HTTP|") {
		t.Errorf("Found duplicate label in edge:\n%s", mermaid)
	}
}
