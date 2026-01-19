package runner

import (
	"recac/internal/utils"
	"testing"
)

func BenchmarkCleanJSON(b *testing.B) {
	input := "```json\n" + `[
		{
			"category": "ui",
			"description": "Verify UI",
			"steps": ["Check UI"],
			"passes": false
		}
	]` + "\n```"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		utils.CleanJSONBlock(input)
	}
}

func BenchmarkCleanJSON_NoMarkdown(b *testing.B) {
	input := `[
		{
			"category": "ui",
			"description": "Verify UI",
			"steps": ["Check UI"],
			"passes": false
		}
	]`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		utils.CleanJSONBlock(input)
	}
}
