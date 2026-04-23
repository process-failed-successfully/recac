package agent

import (
	"os/exec"
)

// execCommandContext allows mocking of exec.CommandContext for testing.
var execCommandContext = exec.CommandContext
