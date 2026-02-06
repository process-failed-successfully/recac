package main

// ticketNode represents a hierarchical task/ticket structure.
type ticketNode struct {
	Title              string       `json:"title"`
	Description        string       `json:"description"`
	Type               string       `json:"type"`
	BlockedBy          []string     `json:"blocked_by"`
	AcceptanceCriteria []string     `json:"acceptance_criteria"`
	Children           []ticketNode `json:"children"`
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
