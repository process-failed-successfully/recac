package orchestrator

import (
	"strings"
	"testing"
)

func BenchmarkExtractRequiredFeatures(b *testing.B) {
	// Create a sample text with multiple features to trigger the inner loop compilation
	var sb strings.Builder
	sb.WriteString("Some description here.\n\nREQUIRED FEATURES:\n")
	for i := 0; i < 100; i++ {
		sb.WriteString("- Feature number ")
		sb.WriteString(string(rune(i))) // Just some variation
		sb.WriteString(" description goes here and needs slugification!!\n")
	}
	text := sb.String()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractRequiredFeatures(text)
	}
}
