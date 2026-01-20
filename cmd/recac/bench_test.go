package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// TestBenchHelperProcess mocks 'go test -bench' output
func TestBenchHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_BENCH_HELPER_PROCESS") != "1" {
		return
	}
	// Print mock benchmark output
	fmt.Println("goos: linux")
	fmt.Println("goarch: amd64")
	fmt.Println("pkg: recac/cmd/recac")
	fmt.Println("BenchmarkMyFunc-8   	10000000	       123.45 ns/op	      10 B/op	       1 allocs/op")
	fmt.Println("BenchmarkOther-8    	 5000000	       200.00 ns/op")
	fmt.Println("PASS")
	fmt.Println("ok  	recac/cmd/recac	1.234s")
	os.Exit(0)
}

func TestParseBenchOutput(t *testing.T) {
	output := `
goos: linux
goarch: amd64
pkg: recac/cmd/recac
BenchmarkOne-8   	10000000	       100 ns/op	      10 B/op	       1 allocs/op
BenchmarkTwo-8   	 5000000	       200.50 ns/op	      20 B/op	       2 allocs/op
PASS
ok  	recac/cmd/recac	1.234s
`
	results, err := parseBenchOutput(output)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}

	if results[0].Name != "BenchmarkOne" || results[0].NsPerOp != 100 {
		t.Errorf("Unexpected result[0]: %+v", results[0])
	}
	if results[1].Name != "BenchmarkTwo" || results[1].NsPerOp != 200.5 {
		t.Errorf("Unexpected result[1]: %+v", results[1])
	}
}

func TestBenchCommand(t *testing.T) {
	// Setup mocks
	originalExec := benchExecCommand
	benchExecCommand = func(name string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestBenchHelperProcess", "--"}
		cs = append(cs, args...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_BENCH_HELPER_PROCESS=1"}
		return cmd
	}
	defer func() { benchExecCommand = originalExec }()

	// Setup temp dir for config
	tmpDir := t.TempDir()
	benchFile = filepath.Join(tmpDir, "benchmarks.json")

	// Ensure flags are reset
	benchSave = true
	benchCompare = false
	benchThreshold = 10.0

	// 1. First Run (Save)
	cmd := benchCmd
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)

	err := runBench(cmd, []string{"."})
	if err != nil {
		t.Fatalf("First run failed: %v", err)
	}

	output := outBuf.String()
	if !strings.Contains(output, "BenchmarkMyFunc") {
		t.Errorf("Output missing benchmark name: %s", output)
	}
	if !strings.Contains(output, "Results saved") {
		t.Errorf("Output missing saved message: %s", output)
	}

	// Verify file created
	if _, err := os.Stat(benchFile); os.IsNotExist(err) {
		t.Fatalf("Benchmark file not created")
	}

	// 2. Second Run (Compare)
	benchCompare = true
	outBuf.Reset()

	// Mock a slightly slower run by changing the helper?
	// The helper is static in this simple setup.
	// So we should see 0% diff or similar.
	// Actually, let's just run it again and verify it compares.

	err = runBench(cmd, []string{"."})
	if err != nil {
		t.Fatalf("Second run failed: %v", err)
	}

	output = outBuf.String()
	if !strings.Contains(output, "DIFF %") {
		t.Errorf("Output missing comparison table: %s", output)
	}
	if !strings.Contains(output, "+0.00%") { // Expect exact match
		t.Errorf("Expected 0%% diff, got output: %s", output)
	}
}

func TestBenchComparisonLogic(t *testing.T) {
	// Test the printComparison logic directly via a fake command execution
	// or just by checking logic if it was exported.
	// Since printComparison writes to cmd.OutOrStdout(), we can test it.

	prev := []BenchResult{
		{Name: "BenchA", NsPerOp: 100},
		{Name: "BenchB", NsPerOp: 100},
	}

	curr := []BenchResult{
		{Name: "BenchA", NsPerOp: 120}, // 20% slower (Fail)
		{Name: "BenchB", NsPerOp: 80},  // 20% faster (Improve)
		{Name: "BenchC", NsPerOp: 50},  // New
	}

	cmd := &cobra.Command{}
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)

	printComparison(cmd, curr, prev, 10.0)

	output := outBuf.String()

	if !strings.Contains(output, "FAIL 🔴") {
		t.Errorf("Expected BenchA to fail: %s", output)
	}
	if !strings.Contains(output, "IMPR 🟢") {
		t.Errorf("Expected BenchB to improve: %s", output)
	}
	if !strings.Contains(output, "NEW") {
		t.Errorf("Expected BenchC to be new: %s", output)
	}
}
