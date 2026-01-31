package runner

import (
	"recac/internal/agent"
	"recac/internal/docker"
	"recac/internal/security"
	"testing"

	"github.com/spf13/viper"
)

func TestSession_ScannerInitialization(t *testing.T) {
	d, _ := docker.NewMockClient()
	a := &agent.MockAgent{}
	workspace := t.TempDir()

	// Case 0: Default (No Viper config set) - Should be Enabled
	// Ensure viper is clean/unset for this key
	// Viper is global, so we can't easily unset. But we can set to nil? No.
	// We rely on other tests not messing it up, or we assume it's unset initially.
	// But since I set it in other tests...
	// I'll skip "unset" verification since Viper state is sticky in unit tests.
	// Instead, I'll verifying explicit setting works.

	// Case 1: Explicitly Enabled - Provider not mock
	viper.Set("security.scan_enabled", true)
	s1 := NewSession(d, a, workspace, "image", "proj", "openrouter", "model", 1)
	if s1.Scanner == nil {
		t.Error("Scanner should be initialized when explicitly enabled")
	}

	// Case 2: Disabled via Config
	viper.Set("security.scan_enabled", false)
	s2 := NewSession(d, a, workspace, "image", "proj", "openrouter", "model", 1)
	if s2.Scanner != nil {
		t.Error("Scanner should be nil when disabled in config")
	}

	// Case 3: Mock Provider (Should be disabled regardless of config)
	viper.Set("security.scan_enabled", true)
	s3 := NewSession(d, a, workspace, "image", "proj", "mock", "model", 1)
	if s3.Scanner != nil {
		t.Error("Scanner should be nil for mock provider")
	}
}

// Ensure interface compatibility
var _ security.Scanner = (*security.RegexScanner)(nil)
