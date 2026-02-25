package architecture

import (
	"fmt"
	"strings"
)

// Violation represents an architectural rule violation.
type Violation struct {
	SourceComponent string
	SourcePackage   string
	TargetComponent string
	TargetPackage   string
	Message         string
}

func (v Violation) String() string {
	return fmt.Sprintf("[%s -> %s] %s", v.SourceComponent, v.TargetComponent, v.Message)
}

// Verifier checks if code dependencies match the architecture.
type Verifier struct {
	Arch *SystemArchitecture
}

// NewVerifier creates a new Verifier.
func NewVerifier(arch *SystemArchitecture) *Verifier {
	return &Verifier{Arch: arch}
}

// Verify checks the dependencies against the architecture rules.
// deps is a map of SourcePackage -> []TargetPackage
func (v *Verifier) Verify(deps map[string][]string) ([]Violation, error) {
	var violations []Violation

	// 1. Build Component Map for O(1) lookup
	// ComponentID -> Component
	compMap := make(map[string]Component)
	for _, c := range v.Arch.Components {
		compMap[c.ID] = c
	}

	// 2. Build Allowed Dependencies Map
	// SourceID -> Set[TargetID]
	allowed := make(map[string]map[string]bool)
	for _, c := range v.Arch.Components {
		if allowed[c.ID] == nil {
			allowed[c.ID] = make(map[string]bool)
		}
		// Allow self (internal dependencies)
		allowed[c.ID][c.ID] = true

		// Add Consumes
		for _, input := range c.Consumes {
			if input.Source != "" {
				allowed[c.ID][input.Source] = true
			}
		}
	}

	// 3. Check Dependencies
	for srcPkg, targets := range deps {
		srcID := v.matchComponent(srcPkg)
		if srcID == "" {
			continue // Source not part of any component
		}

		for _, tgtPkg := range targets {
			tgtID := v.matchComponent(tgtPkg)
			if tgtID == "" {
				continue // Target not part of any component (e.g. libs)
			}

			// If same component, allowed
			if srcID == tgtID {
				continue
			}

			// Check if allowed
			if !allowed[srcID][tgtID] {
				violations = append(violations, Violation{
					SourceComponent: srcID,
					SourcePackage:   srcPkg,
					TargetComponent: tgtID,
					TargetPackage:   tgtPkg,
					Message:         fmt.Sprintf("Package '%s' imports '%s', but component '%s' does not consume '%s'.", srcPkg, tgtPkg, srcID, tgtID),
				})
			}
		}
	}

	return violations, nil
}

// matchComponent attempts to map a package path to a Component ID.
// Heuristic: Component ID is a path segment in the package path.
// e.g. "github.com/org/repo/internal/auth" matches "auth" or "auth-service"
// We iterate components and check if their ID (normalized) is present in path.
// This is simplistic but works for well-structured repos.
func (v *Verifier) matchComponent(pkgPath string) string {
	// Sort components by ID length descending to match specific (longer) names first
	// e.g. match "user-service" before "user"
	// But optimizing that might be overkill for now.

	// Normalize path separators
	pkgPath = strings.ReplaceAll(pkgPath, "\\", "/")
	parts := strings.Split(pkgPath, "/")

	for _, c := range v.Arch.Components {
		// Normalize ID?
		// e.g. "user-service" -> match directory "user-service" or "user_service"
		id := c.ID

		// Exact match in path segments
		for _, part := range parts {
			if part == id {
				return c.ID
			}
			// Try snake_case vs kebab-case mismatch?
			// If ID is "user-service", allow "user_service" in path
			normalizedPart := strings.ReplaceAll(part, "_", "-")
			normalizedID := strings.ReplaceAll(id, "_", "-")
			if normalizedPart == normalizedID {
				return c.ID
			}
		}
	}
	return ""
}
