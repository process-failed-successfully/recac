package utils

import (
	"github.com/pmezard/go-difflib/difflib"
)

// GenerateDiff generates a unified diff between original and improved code.
func GenerateDiff(filename, original, improved string) (string, error) {
	labelOrig := filename
	if labelOrig == "" {
		labelOrig = "original"
	}
	labelNew := labelOrig + " (improved)"

	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(original),
		B:        difflib.SplitLines(improved),
		FromFile: labelOrig,
		ToFile:   labelNew,
		Context:  3,
	}

	text, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		return "", err
	}

	if text == "" {
		return "No changes.\n", nil
	}

	return text, nil
}
