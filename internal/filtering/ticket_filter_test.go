package filtering

import (
	"testing"
)

func TestFilterTickets(t *testing.T) {
	tests := []struct {
		name    string
		tickets []Ticket
		want    int
	}{
		{
			name: "Filter tickets with Ready state and recac-agent label",
			tickets: []Ticket{
				{Key: "TICKET-1", Status: "Ready", Labels: []string{"recac-agent", "bug"}},
				{Key: "TICKET-2", Status: "In Progress", Labels: []string{"recac-agent"}},
				{Key: "TICKET-3", Status: "Ready", Labels: []string{"bug"}},
				{Key: "TICKET-4", Status: "Ready", Labels: []string{"recac-agent"}},
			},
			want: 2,
		},
		{
			name: "No matching tickets",
			tickets: []Ticket{
				{Key: "TICKET-1", Status: "In Progress", Labels: []string{"bug"}},
				{Key: "TICKET-2", Status: "Done", Labels: []string{"recac-agent"}},
			},
			want: 0,
		},
		{
			name: "Empty ticket list",
			tickets: []Ticket{},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FilterTickets(tt.tickets)
			if len(got) != tt.want {
				t.Errorf("FilterTickets() = %d tickets, want %d", len(got), tt.want)
			}
		})
	}
}

func TestIsReady(t *testing.T) {
	tests := []struct {
		name   string
		ticket Ticket
		want   bool
	}{
		{"Ready ticket", Ticket{Status: "Ready"}, true},
		{"In Progress ticket", Ticket{Status: "In Progress"}, false},
		{"Case insensitive Ready", Ticket{Status: "ready"}, true},
		{"Case insensitive READY", Ticket{Status: "READY"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isReady(tt.ticket); got != tt.want {
				t.Errorf("isReady() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasRecacAgentLabel(t *testing.T) {
	tests := []struct {
		name   string
		ticket Ticket
		want   bool
	}{
		{"Has recac-agent label", Ticket{Labels: []string{"recac-agent", "bug"}}, true},
		{"Has recac-agent label only", Ticket{Labels: []string{"recac-agent"}}, true},
		{"No recac-agent label", Ticket{Labels: []string{"bug", "feature"}}, false},
		{"Empty labels", Ticket{Labels: []string{}}, false},
		{"Case insensitive recac-agent", Ticket{Labels: []string{"RECAC-AGENT"}}, true},
		{"Case insensitive Recac-Agent", Ticket{Labels: []string{"Recac-Agent"}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasRecacAgentLabel(tt.ticket); got != tt.want {
				t.Errorf("hasRecacAgentLabel() = %v, want %v", got, tt.want)
			}
		})
	}
}
