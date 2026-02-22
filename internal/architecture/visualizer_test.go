package architecture

import (
	"strings"
	"testing"
)

func TestGenerateMermaid(t *testing.T) {
	arch := &SystemArchitecture{
		Components: []Component{
			{
				ID:   "Order Service",
				Type: "service",
				Consumes: []Input{
					{Source: "Web UI", Type: "OrderRequest"},
				},
				Produces: []Output{
					{Target: "Payment DB", Type: "PaymentRecord"},
				},
			},
			{
				ID:   "Web UI",
				Type: "frontend",
			},
			{
				ID:   "Payment DB",
				Type: "database",
			},
			{
				ID:   "Queue",
				Type: "queue",
				Consumes: []Input{
					{Source: "Order Service", Type: "NotificationEvent"},
				},
			},
		},
	}

	mermaid := GenerateMermaid(arch)

	// Check Header
	if !strings.Contains(mermaid, "graph TD") {
		t.Errorf("Expected 'graph TD', got %s", mermaid)
	}

	// Check Nodes
	// "Web UI" -> Web_UI
	if !strings.Contains(mermaid, "Web_UI((\"Web UI\"))") {
		t.Errorf("Missing Web UI node (frontend style)")
	}
	// "Payment DB" -> Payment_DB
	if !strings.Contains(mermaid, "Payment_DB[(\"Payment DB\")]") {
		t.Errorf("Missing Payment DB node (database style)")
	}
	// "Queue" -> Queue
	if !strings.Contains(mermaid, "Queue{{\"Queue\"}}") {
		t.Errorf("Missing Queue node (queue style)")
	}
	// "Order Service" -> Order_Service
	if !strings.Contains(mermaid, "Order_Service[\"Order Service\"]") {
		t.Errorf("Missing Order Service node (default style)")
	}

	// Check Edges
	// Web UI -> Order Service (Consumes)
	if !strings.Contains(mermaid, "Web_UI -->|OrderRequest| Order_Service") {
		t.Errorf("Missing edge Web UI -> Order Service")
	}

	// Order Service -> Payment DB (Produces)
	if !strings.Contains(mermaid, "Order_Service -->|PaymentRecord| Payment_DB") {
		t.Errorf("Missing edge Order Service -> Payment DB")
	}

	// Order Service -> Queue (Consumes)
	if !strings.Contains(mermaid, "Order_Service -->|NotificationEvent| Queue") {
		t.Errorf("Missing edge Order Service -> Queue")
	}
}

func TestGenerateMermaid_Empty(t *testing.T) {
	arch := &SystemArchitecture{
		Components: []Component{},
	}

	mermaid := GenerateMermaid(arch)

	if !strings.Contains(mermaid, "graph TD") {
		t.Errorf("Expected 'graph TD'")
	}
	if len(strings.Split(mermaid, "\n")) > 2 { // header + empty line?
		// Actually, depending on newlines, it might be just "graph TD\n"
		if strings.TrimSpace(mermaid) != "graph TD" {
			t.Errorf("Expected only header for empty architecture, got:\n%s", mermaid)
		}
	}
}
