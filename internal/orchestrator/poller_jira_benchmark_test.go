package orchestrator

import (
	"strings"
	"testing"
)

func BenchmarkExtractRequiredFeatures(b *testing.B) {
	// Prepare a reasonably large description with features
	var sb strings.Builder
	sb.WriteString("Some introductory text here.\n")
	sb.WriteString("REQUIRED FEATURES:\n")
	for i := 0; i < 100; i++ {
		sb.WriteString("- Feature number " + string(rune(i)) + " which is very important\n")
	}
	sb.WriteString("\nSome footer text.\n")
	description := sb.String()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractRequiredFeatures(description)
	}
}
