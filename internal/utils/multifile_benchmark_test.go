package utils

import (
	"testing"
)

func BenchmarkParseFileBlocks(b *testing.B) {
	input := `<file path="test.go">
package main
func main() {}
</file>
<file path="README.md">
# Hello
</file>`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParseFileBlocks(input)
	}
}
