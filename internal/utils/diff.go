package utils

import (
	"github.com/pmezard/go-difflib/difflib"
)

// GenerateDiff creates a unified diff between two contents.
func GenerateDiff(labelA, contentA, labelB, contentB string) (string, error) {
	if labelA == "" {
		labelA = "original"
	}
	if labelB == "" {
		labelB = "improved"
	}

	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(contentA),
		B:        difflib.SplitLines(contentB),
		FromFile: labelA,
		ToFile:   labelB,
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
