package utils

// DefaultIgnoreMap returns a map of common directories and files to ignore during scans.
func DefaultIgnoreMap() map[string]bool {
	return map[string]bool{
		".git":         true,
		"node_modules": true,
		"vendor":       true,
		"dist":         true,
		"build":        true,
		".recac":       true,
		".idea":        true,
		".vscode":      true,
		"bin":          true,
		"obj":          true,
		"__pycache__":  true,
		"TODO.md":      true,
	}
}
