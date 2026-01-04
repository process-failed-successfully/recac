package filtering

import (
	"log"
	"strings"
)

// Ticket represents a Jira ticket
type Ticket struct {
	Key     string
	Summary string
	Status  string
	Labels  []string
}

// FilterTickets filters tickets by Ready state and recac-agent label
func FilterTickets(tickets []Ticket) []Ticket {
	var filtered []Ticket

	for _, ticket := range tickets {
		if isReady(ticket) && hasRecacAgentLabel(ticket) {
			filtered = append(filtered, ticket)
		}
	}

	log.Printf("Filtered %d tickets (Ready state and recac-agent label)", len(filtered))
	return filtered
}

// isReady checks if ticket is in Ready state
func isReady(ticket Ticket) bool {
	return strings.EqualFold(ticket.Status, "Ready")
}

// hasRecacAgentLabel checks if ticket has recac-agent label
func hasRecacAgentLabel(ticket Ticket) bool {
	for _, label := range ticket.Labels {
		if strings.EqualFold(label, "recac-agent") {
			return true
		}
	}
	return false
}
