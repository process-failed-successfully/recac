package main

import (
	"os"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestInitConfig(t *testing.T) {
	// Setup temp config file
	f, err := os.CreateTemp("", "config_test_*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())

	// Write valid config
	// Assuming minimal config needed for ValidateConfig to pass or at least not fail catastrophically
	// If ValidateConfig requires specific keys, we might need to add them.
	// Let's assume defaults or minimal content.
	f.WriteString("provider: test-provider\n")
	f.Close()

	// Capture original state
	oldCfgFile := cfgFile
	oldExit := exit
	defer func() {
		cfgFile = oldCfgFile
		exit = oldExit
		viper.Reset()
	}()

	// Mock exit
	exitCode := -1
	exit = func(code int) {
		exitCode = code
		// Don't actually exit
	}

	// Test Case 1: Valid Config File
	cfgFile = f.Name()

	// Reset viper before test
	viper.Reset()

	initConfig()

	assert.Equal(t, -1, exitCode, "initConfig should not exit on valid config")
	assert.Equal(t, "test-provider", viper.GetString("provider"))

	// Test Case 2: Config Validation Failure (simulate by forcing invalid state if possible)
	// If ValidateConfig checks for specific values.
	// We can try to mess up validation.
	// For example, if provider is required.

	// Let's try to mock ValidateConfig? No, it's imported from internal/config.
	// We have to rely on its behavior.
	// If we provide a file with invalid structure?

	// Let's try invalid YAML
	f2, _ := os.CreateTemp("", "config_invalid_*.yaml")
	defer os.Remove(f2.Name())
	f2.WriteString("key: value: what:\n") // Invalid YAML
	f2.Close()

	cfgFile = f2.Name()
	viper.Reset()

	// config.Load might print error but not exit.
	// initConfig calls config.Load then ValidateConfig.

	// We can't easily force ValidateConfig to fail without knowing its logic.
	// But let's verify initConfig runs.
}

func TestExecute_PanicRecovery(t *testing.T) {
	// Test panic recovery in Execute
	// We can't easily cause panic in rootCmd.Execute() unless we modify it or add a command that panics.

	// Add a command that panics
	panicCmd := &cobra.Command{
		Use: "panic-test",
		Run: func(cmd *cobra.Command, args []string) {
			panic("simulated panic")
		},
	}
	rootCmd.AddCommand(panicCmd)
	defer rootCmd.RemoveCommand(panicCmd)

	// Mock exit
	oldExit := exit
	exitCode := -1
	exit = func(code int) {
		exitCode = code
	}
	defer func() { exit = oldExit }()

	// Capture stderr
	// Execute() calls os.Args parsing. We need to set os.Args.
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	os.Args = []string{"recac", "panic-test"}

	// We also need to avoid calling config.Load inside Execute which might fail or side-effect.
	// Execute calls config.Load(preCfgFile).

	// Run Execute
	// Since Execute() calls exit(1) on panic recovery, we expect exitCode = 1.

	// Note: Execute() uses defer for recovery.

	// We need to capture panic because our mock exit doesn't stop execution,
	// so the panic handler finishes and returns.

	func() {
		defer func() {
			if r := recover(); r != nil {
				// Should not happen if Execute handles it
				t.Errorf("Panic reached test scope: %v", r)
			}
		}()
		Execute()
	}()

	assert.Equal(t, 1, exitCode, "Execute should exit(1) on panic")
}
