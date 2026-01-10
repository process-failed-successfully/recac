package main

import "strings"

// isSensitive checks if a key is sensitive.
func isSensitive(key string) bool {
	lowerKey := strings.ToLower(key)
	return strings.Contains(lowerKey, "key") ||
		strings.Contains(lowerKey, "token") ||
		strings.Contains(lowerKey, "secret")
}
