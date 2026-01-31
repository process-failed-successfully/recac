package jira

import (
	"fmt"
	"recac/internal/db"
	"regexp"
	"strings"
)

// ExtractRequiredFeatures parses text for required features or acceptance criteria sections.
func ExtractRequiredFeatures(text string) []db.Feature {
	// Look for REQUIRED FEATURES: or ACCEPTANCE CRITERIA: block
	// Regex matches headers case-insensitively
	// Then captures lines starting with "- " or "* " until a blank line or new section
	var features []db.Feature

	lines := strings.Split(text, "\n")
	inSection := false

	headerRegex := regexp.MustCompile(`(?i)^(REQUIRED FEATURES|ACCEPTANCE CRITERIA):?\s*$`)

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if headerRegex.MatchString(line) {
			inSection = true
			continue
		}

		if inSection {
			if line == "" || strings.HasPrefix(line, "#") || (strings.HasSuffix(line, ":") && !strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "*")) {
				// End of section (empty line, comment, or new header)
				if line != "" {
					if strings.Contains(line, ":") {
						break
					}
				}
			}

			if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
				// Extract feature description
				desc := strings.TrimSpace(line[2:])
				// Create a simplified Feature
				slug := strings.ToLower(desc)
				reg, _ := regexp.Compile("[^a-z0-9]+")
				slug = reg.ReplaceAllString(slug, "-")
				slug = strings.Trim(slug, "-")
				if len(slug) > 30 {
					slug = slug[:30]
				}

				f := db.Feature{
					ID:          fmt.Sprintf("req-%s", slug),
					Description: desc,
					Category:    "functional",
					Priority:    "critical",
					Status:      "pending",
				}
				features = append(features, f)
			}
		}
	}
	return features
}
