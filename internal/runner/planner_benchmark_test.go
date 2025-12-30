package runner

import (
	"testing"
)

func BenchmarkCleanJSON(b *testing.B) {
	input := "```json\n" + `[
		{
			"category": "cli",
			"description": "Verify version command",
			"steps": ["Run version"],
			"passes": false
		}
	]` + "\n```"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cleanJSON(input)
	}
}
