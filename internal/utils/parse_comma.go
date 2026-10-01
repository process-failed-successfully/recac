package utils

import "strings"

// ParseCommaSeparated parses a comma-separated string, trimming spaces
// and ignoring empty elements, without allocating intermediate slices like strings.Split does.
// ⚡ Bolt: Avoid strings.Split overhead for comma-separated parsing.
func ParseCommaSeparated(s string) []string {
	if s == "" {
		return nil
	}

	count := strings.Count(s, ",")
	if count == 0 {
		trimmed := strings.TrimSpace(s)
		if trimmed == "" {
			return nil
		}
		return []string{trimmed}
	}

	res := make([]string, 0, count+1)

	for {
		idx := strings.IndexByte(s, ',')
		if idx == -1 {
			trimmed := strings.TrimSpace(s)
			if trimmed != "" {
				res = append(res, trimmed)
			}
			break
		}

		trimmed := strings.TrimSpace(s[:idx])
		if trimmed != "" {
			res = append(res, trimmed)
		}
		s = s[idx+1:]
	}

	return res
}
