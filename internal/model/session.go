package model

import (
	"recac/internal/agent"
	"time"
)

// UnifiedSession represents both a local session and a remote K8s pod for display purposes.
type UnifiedSession struct {
	Name         string
	Status       string
	StartTime    time.Time
	LastActivity time.Time
	EndTime      time.Time
	Duration     time.Duration
	Location     string
	Cost         float64
	HasCost      bool
	Tokens       agent.TokenUsage
	Goal         string
	CPU          string
	Memory       string
}

// PsFilters holds the filter values for the ps command.
type PsFilters struct {
	Status string
	Since  string
	Stale  string
	Remote bool
}
