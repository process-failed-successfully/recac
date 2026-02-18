package security

import (
	"strings"
	"testing"
)

func BenchmarkScanner_Scan(b *testing.B) {
	// Create a large content with many matches
	// Each "match" line has ~50 chars, plus a newline
	// We'll create 10,000 lines
	// Increase match density to 10%
	var sb strings.Builder
	for i := 0; i < 10000; i++ {
		if i%10 == 0 {
			// Insert a match every 10 lines
			sb.WriteString("var key = \"AKIAIOSFODNN7EXAMPLE\"\n")
		} else {
			sb.WriteString("some innocuous code here that is safe and sound\n")
		}
	}
	content := sb.String()
	scanner := NewRegexScanner()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = scanner.Scan(content)
	}
}
