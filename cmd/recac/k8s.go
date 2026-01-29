package main

import (
	"context"
	"fmt"
	"text/tabwriter"

	"recac/internal/utils"

	"github.com/spf13/cobra"
)

var (
	k8sTailLines int64
)

var k8sCmd = &cobra.Command{
	Use:   "k8s",
	Short: "Manage Kubernetes agents",
	Long:  `Manage and inspect distributed agents running in Kubernetes.`,
}

var k8sListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls", "status"},
	Short:   "List running agent pods",
	RunE:    runK8sList,
}

var k8sLogsCmd = &cobra.Command{
	Use:   "logs [pod_name_or_ticket_id]",
	Short: "Get logs from an agent pod",
	Args:  cobra.ExactArgs(1),
	RunE:  runK8sLogs,
}

var k8sCleanupCmd = &cobra.Command{
	Use:     "cleanup",
	Aliases: []string{"prune"},
	Short:   "Delete completed or failed agent pods",
	RunE:    runK8sCleanup,
}

func init() {
	rootCmd.AddCommand(k8sCmd)
	k8sCmd.AddCommand(k8sListCmd)
	k8sCmd.AddCommand(k8sLogsCmd)
	k8sCmd.AddCommand(k8sCleanupCmd)

	k8sLogsCmd.Flags().Int64VarP(&k8sTailLines, "tail", "t", 100, "Number of lines to show from the end of the logs")
}

func runK8sList(cmd *cobra.Command, args []string) error {
	client, err := k8sClientFactory()
	if err != nil {
		return fmt.Errorf("failed to create k8s client: %w", err)
	}

	ctx := context.Background()
	pods, err := client.ListPods(ctx, "app=recac-agent")
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No agent pods found.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tTICKET\tAGE")

	for _, pod := range pods {
		age := utils.FormatSince(pod.CreationTimestamp.Time)
		ticket := pod.Labels["ticket"]
		if ticket == "" {
			ticket = "N/A"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", pod.Name, pod.Status.Phase, ticket, age)
	}
	w.Flush()
	return nil
}

func runK8sLogs(cmd *cobra.Command, args []string) error {
	target := args[0]
	client, err := k8sClientFactory()
	if err != nil {
		return fmt.Errorf("failed to create k8s client: %w", err)
	}

	ctx := context.Background()

	// 1. Resolve Pod Name
	podName := target

	// Optimization: If it starts with "recac-agent-", assume it is a pod name.
	// Otherwise, check if it matches a ticket.
	pods, err := client.ListPods(ctx, "app=recac-agent")
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	found := false
	for _, pod := range pods {
		if pod.Name == target {
			found = true
			break
		}
		if pod.Labels["ticket"] == target {
			podName = pod.Name
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("pod or ticket '%s' not found", target)
	}

	// 2. Fetch Logs
	logs, err := client.GetPodLogs(ctx, podName, k8sTailLines)
	if err != nil {
		return fmt.Errorf("failed to get logs for %s: %w", podName, err)
	}

	fmt.Fprint(cmd.OutOrStdout(), logs)
	return nil
}

func runK8sCleanup(cmd *cobra.Command, args []string) error {
	client, err := k8sClientFactory()
	if err != nil {
		return fmt.Errorf("failed to create k8s client: %w", err)
	}

	ctx := context.Background()
	pods, err := client.ListPods(ctx, "app=recac-agent")
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	deleted := 0
	for _, pod := range pods {
		phase := pod.Status.Phase
		// Also clean up unknown?
		if phase == "Succeeded" || phase == "Failed" || phase == "Unknown" {
			fmt.Fprintf(cmd.OutOrStdout(), "Deleting %s (%s)...\n", pod.Name, phase)
			if err := client.DeletePod(ctx, pod.Name); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Failed to delete %s: %v\n", pod.Name, err)
			} else {
				deleted++
			}
		}
	}

	if deleted == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No cleanup needed.")
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "Deleted %d pods.\n", deleted)
	}

	return nil
}
