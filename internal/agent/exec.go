package agent

import (
	"os/exec"
)

// execCommandContext allows mocking of exec.CommandContext for testing.
var execCommandContext = exec.CommandContext

// execCommand allows mocking of exec.Command for testing.
var execCommand = exec.Command
