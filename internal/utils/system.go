package utils

import (
	"fmt"
	"os/exec"
)

// CheckGoInstalled checks if the Go binary is available in the system PATH.
func CheckGoInstalled() error {
	_, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("go binary not found in PATH")
	}
	return nil
}

// CheckDockerRunning checks if the Docker daemon is running by executing 'docker info'.
func CheckDockerRunning() error {
	cmd := exec.Command("docker", "info")
	return cmd.Run()
}
