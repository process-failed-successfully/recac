package model

import (
	"recac/internal/agent"
	"time"
)

// UnifiedSession represents a session from any source (local, k8s, etc.)
// for consistent display in UIs.
type UnifiedSession struct {
	Name      string
	Status    string
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
	Location  string
	Cost      float64
	HasCost   bool
	Tokens    agent.TokenUsage
}
