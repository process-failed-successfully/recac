package vuln

import "time"

// Package represents a dependency package with version.
type Package struct {
	Name      string
	Version   string
	Ecosystem string // "Go", "npm", "PyPI", etc.
}

// Vulnerability represents a security vulnerability found in a package.
type Vulnerability struct {
	ID          string    `json:"id"`
	Summary     string    `json:"summary"`
	Details     string    `json:"details"`
	Severity    string    `json:"severity"` // e.g. "CRITICAL", "HIGH", "MODERATE", "LOW"
	References  []string  `json:"references"`
	Published   time.Time `json:"published"`
	PackageName string    `json:"package_name"`
	Ecosystem   string    `json:"ecosystem"`
}

// Scanner scans for vulnerabilities.
type Scanner interface {
	Scan(packages []Package) ([]Vulnerability, error)
}

// Parser parses a dependency file.
type Parser interface {
	Parse(path string) ([]Package, error)
}
