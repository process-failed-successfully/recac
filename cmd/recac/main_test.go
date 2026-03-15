package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestMainPanic(t *testing.T) {
	if os.Getenv("CRASH_TEST") == "1" {
		ExecuteFunc = func() {
			panic("forced panic in main")
		}
		main()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestMainPanic")
	cmd.Env = append(os.Environ(), "CRASH_TEST=1")
	out, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatalf("process ran with err %v, want exit status 1", err)
	}

	output := string(out)
	if !strings.Contains(output, "CRITICAL ERROR: Application Panic") {
		t.Errorf("Expected panic message in stderr, got: %s", output)
	}
}

func TestMainNormal(t *testing.T) {
	if os.Getenv("NORMAL_TEST") == "1" {
		oldArgs := os.Args
		os.Args = []string{"recac", "--help"}
		defer func() { os.Args = oldArgs }()
		main()
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestMainNormal")
	cmd.Env = append(os.Environ(), "NORMAL_TEST=1")
	out, err := cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("process ran with err %v, output: %s", err, string(out))
	}
}
