package orchestrator

import (
	"testing"
)

func BenchmarkSanitizeName(b *testing.B) {
	name := "Build Application for v1.0"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sanitizeName(name)
	}
}
