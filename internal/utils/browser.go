package utils

import (
	"fmt"
	"os/exec"
	"runtime"
)

// execCommand is a variable to allow mocking in tests.
var execCommand = exec.Command

// OpenBrowser opens the specified URL in the default browser of the user.
func OpenBrowser(url string) error {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = execCommand("xdg-open", url).Start()
	case "windows":
		err = execCommand("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = execCommand("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		return fmt.Errorf("failed to open browser: %w", err)
	}

	return nil
}
