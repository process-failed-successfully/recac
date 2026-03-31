package main

import (
	"strings"
	"testing"
)

func BenchmarkToLower(b *testing.B) {
	t1 := "SomeTag"
	lowerTag := strings.ToLower("sometag")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = strings.ToLower(t1) == lowerTag
	}
}

func BenchmarkEqualFold(b *testing.B) {
	t1 := "SomeTag"
	tag := "sometag"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = strings.EqualFold(t1, tag)
	}
}
