package architecture

import (
	"fmt"
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
// ⚡ Bolt: Replaced string allocation-heavy split and replace calls with a zero-allocation byte iteration.
// Impact: Reduces string allocations per call and improves execution time significantly.
func (v *Verifier) matchComponent(pkgPath string) string {
	// Sort components by ID length descending to match specific (longer) names first
	// e.g. match "user-service" before "user"
	// But optimizing that might be overkill for now.

	for _, c := range v.Arch.Components {
		id := c.ID

		start := 0
		for i := 0; i <= len(pkgPath); i++ {
			// Treat both forward and backward slashes as separators
			if i == len(pkgPath) || pkgPath[i] == '/' || pkgPath[i] == '\\' {
				if start < i {
					part := pkgPath[start:i]

					// Fast length check first
					if len(part) == len(id) {
						match := true
						for j := 0; j < len(part); j++ {
							// Allow interchangeability of snake_case and kebab-case
							if part[j] != id[j] && !(part[j] == '_' && id[j] == '-') && !(part[j] == '-' && id[j] == '_') {
								match = false
								break
							}
						}
						if match {
							return c.ID
						}
					}
				}
				start = i + 1
			}
		}
	}
	return ""
}
