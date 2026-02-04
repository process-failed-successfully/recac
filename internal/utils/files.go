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

// IsBinaryExt checks if the file extension corresponds to a binary file.
func IsBinaryExt(ext string) bool {
	switch ext {
	case ".exe", ".dll", ".so", ".dylib", ".bin", ".jpg", ".png", ".gif", ".pdf", ".zip", ".tar", ".gz", ".iso", ".class", ".jar":
		return true
	}
	return false
}

// IsBinaryContent checks the first few bytes of a file to see if it contains null bytes, indicating binary content.
func IsBinaryContent(content []byte) bool {
	limit := 512
	if len(content) < limit {
		limit = len(content)
	}
	for i := 0; i < limit; i++ {
		if content[i] == 0 {
			return true
		}
	}
	return false
}
