package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(stopCmd)
}

var stopCmd = &cobra.Command{
	Use:   "stop [session-name]",
	Short: "Stop a running session",
	Long:  `Stop a running session gracefully. Sends SIGTERM first, then SIGKILL if needed.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var sessionName string
		var err error

		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		if len(args) == 0 {
			sessionName, err = interactiveSessionSelect(sm, "running", "Choose a session to stop:")
			if err != nil {
				return err
			}
		} else {
			sessionName = args[0]
		}

		if err := sm.StopSession(sessionName); err != nil {
			// If session not found locally, try K8s
			if strings.Contains(err.Error(), "session not found") || strings.Contains(err.Error(), "no such file or directory") {
				k8sClient, k8sErr := k8sClientFactory()
				if k8sErr == nil {
					pods, listErr := k8sClient.ListPods(context.Background(), "app=recac-agent")
					if listErr == nil {
						for _, pod := range pods {
							// Check if the pod's ticket label matches the requested session name
							if pod.Labels["ticket"] == sessionName {
								if delErr := k8sClient.DeletePod(context.Background(), pod.Name); delErr != nil {
									return fmt.Errorf("found K8s pod for session '%s' but failed to delete: %w", sessionName, delErr)
								}
								fmt.Fprintf(cmd.OutOrStdout(), "K8s pod for session '%s' (pod: %s) deleted successfully\n", sessionName, pod.Name)
								return nil
							}
						}
					}
				}
			}
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Session '%s' stopped successfully\n", sessionName)
		return nil
	},
}
