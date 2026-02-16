package agent

import (
	"strings"
	"testing"
)

func BenchmarkTruncateToTokenLimit_SmallLimit(b *testing.B) {
	line := "short line\n"
	text := strings.Repeat(line, 100000) // ~1.1MB
	maxTokens := 1000                    // Force truncation, small keep, huge drop

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		TruncateToTokenLimit(text, maxTokens)
	}
}

func BenchmarkTruncateToTokenLimit_LargeLimit(b *testing.B) {
	line := "short line\n"
	text := strings.Repeat(line, 100000) // ~1.1MB
	// 1.1MB chars / 4 = ~275k tokens.
	// Let's keep 200k tokens. ~800k chars.
	// maxStartChars ~ 400k.
	// Lines are ~11 chars.
	// Loop would run 400,000 / 11 = ~36,000 times.
	maxTokens := 200000

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		TruncateToTokenLimit(text, maxTokens)
	}
}
