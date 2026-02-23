package migration

import (
	"time"
)

// Migration represents a single migration file or record.
type Migration struct {
	Version   string    `json:"version"` // Timestamp: YYYYMMDDHHMMSS
	Name      string    `json:"name"`
	UpFile    string    `json:"up_file"`
	DownFile  string    `json:"down_file"`
	AppliedAt time.Time `json:"applied_at,omitempty"`
}

// Status represents the current state of migrations.
type Status struct {
	Applied []Migration `json:"applied"`
	Pending []Migration `json:"pending"`
}
