package utils

import (
	"fmt"
	"os/exec"
	"runtime"
)

var execCommand = exec.Command
var runtimeGOOS = runtime.GOOS

func OpenBrowser(url string) error {
	var err error

	switch runtimeGOOS {
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
