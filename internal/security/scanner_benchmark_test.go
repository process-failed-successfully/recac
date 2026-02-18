package security

import (
	"strings"
	"testing"
)

func BenchmarkScanner_Scan(b *testing.B) {
	s := NewRegexScanner()
	// Create a large content with many matches and newlines
	line := "var key = \"AKIAIOSFODNN7EXAMPLE\"\n"
	content := strings.Repeat(line, 1000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := s.Scan(content)
		if err != nil {
			b.Fatal(err)
		}
	}
}
