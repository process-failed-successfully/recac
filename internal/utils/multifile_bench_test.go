package utils

import "testing"

func BenchmarkParseFileBlocks(b *testing.B) {
	input := `<file path="a.txt">
Hello
</file>
Some text in between
<file path="b.txt">
World
</file>
<file path="very_long_file.go">
package main

func main() {
	// this is some content
	println("hello world")
}
</file>`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ParseFileBlocks(input)
	}
}
