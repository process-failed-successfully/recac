package architecture

import (
	"testing"
)

func BenchmarkMatchComponentOrig(b *testing.B) {
	arch := &SystemArchitecture{
		Components: []Component{
			{ID: "auth-service"},
			{ID: "user-service"},
			{ID: "billing-service"},
			{ID: "notification_service"},
			{ID: "frontend"},
		},
	}
	verifier := NewVerifier(arch)
	pkgPath := "github.com/org/repo/internal/user_service/models"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		verifier.matchComponent(pkgPath)
	}
}
