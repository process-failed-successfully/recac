package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFlakyRun(t *testing.T) {
	// Save original flakyExec
	origExec := flakyExec
	defer func() { flakyExec = origExec }()

	callCount := 0
	flakyExec = func(name string, arg ...string) *exec.Cmd {
		callCount++
		exe, _ := os.Executable()
		cmd := exec.Command(exe, "-test.run=TestFlakyHelperProcess", "--")
		cmd.Env = os.Environ()
		cmd.Env = append(cmd.Env, "GO_WANT_HELPER_PROCESS=1")

		// Run 1: Pass
		// Run 2: Fail
		// Run 3: Pass
		if callCount == 2 {
			cmd.Env = append(cmd.Env, "MOCK_FLAKY_OUTCOME=fail")
		} else {
			cmd.Env = append(cmd.Env, "MOCK_FLAKY_OUTCOME=pass")
		}
		return cmd
	}

	// Setup flags
	flakyCount = 3
	flakyJSON = true
	flakyTimeout = 5 * time.Second

	// Create dummy command to capture output
	cmd := &cobra.Command{}
	var outBuf bytes.Buffer
	var errBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&errBuf)

	// Execute
	err := runFlaky(cmd, []string{"./..."})

	// We expect no error in execution, but runFlaky returns error if it finds flaky tests?
	// Yes: "return fmt.Errorf("found %d flaky tests", len(flakyTests))" if JSON is false?
	// Wait, check code:
	// if flakyJSON { ... return enc.Encode(res) }
	// So if JSON is true, it returns the result of encoding.
	// Encoding usually returns nil error.

	require.NoError(t, err)

	// Parse Output
	var res FlakyResult
	err = json.Unmarshal(outBuf.Bytes(), &res)
	require.NoError(t, err, "Failed to parse JSON output: %s", outBuf.String())

	// Validate Stats
	assert.Equal(t, 3, res.Stats.TotalRuns)
	assert.Equal(t, 1, res.Stats.FlakyFound)
	assert.Equal(t, 1, res.Stats.TestsFound)

	// Validate Flaky Test
	require.Len(t, res.FlakyTests, 1)
	ft := res.FlakyTests[0]
	assert.Equal(t, "TestFlaky", ft.Name)
	assert.Equal(t, "example.com/mypkg", ft.Package)
	assert.Equal(t, 2, ft.Pass) // Run 1 & 3
	assert.Equal(t, 1, ft.Fail) // Run 2
}

func TestFlakyHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	// We must exit at the end to act as a subprocess
	defer os.Exit(0)

	outcome := os.Getenv("MOCK_FLAKY_OUTCOME")

	// Print Package Start
	// go test -json output format
	fmt.Println(`{"Time":"2023-01-01T00:00:00Z","Action":"run","Package":"example.com/mypkg","Test":"TestFlaky"}`)

	if outcome == "fail" {
		// Output failure
		fmt.Println(`{"Time":"2023-01-01T00:00:00Z","Action":"output","Package":"example.com/mypkg","Test":"TestFlaky","Output":"FAIL: TestFlaky (0.00s)\n"}`)
		fmt.Println(`{"Time":"2023-01-01T00:00:00Z","Action":"fail","Package":"example.com/mypkg","Test":"TestFlaky"}`)
		// Package fail
		fmt.Println(`{"Time":"2023-01-01T00:00:00Z","Action":"fail","Package":"example.com/mypkg","Elapsed":0.1}`)
	} else {
		// Pass
		fmt.Println(`{"Time":"2023-01-01T00:00:00Z","Action":"pass","Package":"example.com/mypkg","Test":"TestFlaky"}`)
		// Package pass
		fmt.Println(`{"Time":"2023-01-01T00:00:00Z","Action":"pass","Package":"example.com/mypkg","Elapsed":0.1}`)
	}
}
