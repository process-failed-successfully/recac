package architecture

import (
	"testing"
)

func TestVerifier_Verify(t *testing.T) {
	// Setup Architecture
	arch := &SystemArchitecture{
		Components: []Component{
			{
				ID: "auth",
				Consumes: []Input{
					{Source: "db"}, // Auth depends on DB
				},
			},
			{
				ID: "db",
				// DB consumes nothing
			},
			{
				ID: "api",
				Consumes: []Input{
					{Source: "auth"}, // API depends on Auth
				},
			},
		},
	}

	verifier := NewVerifier(arch)

	// Test Cases
	t.Run("Allowed Dependencies", func(t *testing.T) {
		deps := map[string][]string{
			"pkg/auth/login":  {"pkg/db/sql"},
			"pkg/api/handler": {"pkg/auth/login"},
		}

		violations, err := verifier.Verify(deps)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(violations) != 0 {
			t.Errorf("expected 0 violations, got %d: %v", len(violations), violations)
		}
	})

	t.Run("Internal Dependencies", func(t *testing.T) {
		deps := map[string][]string{
			"pkg/auth/login": {"pkg/auth/models"},
		}
		violations, err := verifier.Verify(deps)
		if err != nil {
			t.Fatal(err)
		}
		if len(violations) != 0 {
			t.Errorf("internal dependencies should be allowed")
		}
	})

	t.Run("Forbidden Dependency", func(t *testing.T) {
		deps := map[string][]string{
			"pkg/db/sql": {"pkg/api/handler"}, // DB should not depend on API
		}
		violations, err := verifier.Verify(deps)
		if err != nil {
			t.Fatal(err)
		}
		if len(violations) != 1 {
			t.Fatalf("expected 1 violation, got %d", len(violations))
		}
		v := violations[0]
		if v.SourceComponent != "db" || v.TargetComponent != "api" {
			t.Errorf("violation source/target mismatch: %v", v)
		}
	})

	t.Run("Unknown Component Ignored", func(t *testing.T) {
		deps := map[string][]string{
			"pkg/util/helper": {"pkg/auth/login"}, // util is unknown
		}
		violations, err := verifier.Verify(deps)
		if err != nil {
			t.Fatal(err)
		}
		if len(violations) != 0 {
			t.Errorf("unknown source component should be ignored")
		}
	})

	t.Run("Unknown Target Ignored", func(t *testing.T) {
		deps := map[string][]string{
			"pkg/auth/login": {"github.com/google/uuid"}, // external lib
		}
		violations, err := verifier.Verify(deps)
		if err != nil {
			t.Fatal(err)
		}
		if len(violations) != 0 {
			t.Errorf("unknown target component should be ignored")
		}
	})
}

func TestViolation_String(t *testing.T) {
	v := Violation{
		SourceComponent: "src",
		TargetComponent: "tgt",
		Message:         "msg",
	}
	expected := "[src -> tgt] msg"
	if v.String() != expected {
		t.Errorf("expected %s, got %s", expected, v.String())
	}
}
