package main

import (
	"recac/internal/utils"
)

type ContextOptions = utils.ContextOptions

// GenerateCodebaseContext generates a markdown string containing the file tree and contents
// of the specified roots, respecting ignore patterns and size limits.
func GenerateCodebaseContext(opts ContextOptions) (string, error) {
	return utils.GenerateCodebaseContext(opts)
}
