package analysis

import (
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkGenerateCallGraph(b *testing.B) {
	wd, err := os.Getwd()
	if err != nil {
		b.Fatal(err)
	}
	root := filepath.Join(wd, "../..")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := GenerateCallGraph(root)
		if err != nil {
			b.Fatal(err)
		}
	}
}
