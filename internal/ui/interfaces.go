package ui

// This package defines interfaces to prevent import cycles between the `cmd`
// and `ui` packages.

// Session represents a simplified session object for the UI to display.
// We redefine it here to avoid a direct dependency on the `main` package's
// `unifiedSession`. The loader function will be responsible for the mapping.
type Session struct {
	Name      string
	Status    string
	Location  string
	StartTime string
	Cost      string
	Details   string // A pre-formatted string for the detail view
}

// SessionLoader is a function type that the `cmd` package will implement.
// It allows the UI to request a list of all sessions without knowing how
// they are fetched.
type SessionLoader func() ([]Session, error)
