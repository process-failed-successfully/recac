package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// Confirm displays a prompt to the user and waits for a yes/no response.
func Confirm(cmd *cobra.Command, prompt string, force bool) (bool, error) {
	if force {
		return true, nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%s [y/N]: ", prompt)

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}

	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes", nil
}
