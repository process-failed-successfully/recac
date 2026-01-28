package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

var (
	k8sNamespace string
)

// k8sNativeClientFactory allows mocking in tests.
// Note: k8sClientFactory (in factories.go) returns a wrapper IK8sClient which is too limited.
var k8sNativeClientFactory = func() (kubernetes.Interface, error) {
	// 1. Try In-Cluster Config
	config, err := rest.InClusterConfig()
	if err != nil {
		// 2. Fallback to ~/.kube/config
		var kubeconfig string
		if os.Getenv("KUBECONFIG") != "" {
			kubeconfig = os.Getenv("KUBECONFIG")
		} else if home := homedir.HomeDir(); home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		}

		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
		}
	}
	return kubernetes.NewForConfig(config)
}

var k8sCmd = &cobra.Command{
	Use:   "k8s",
	Short: "Manage RECAC agents in Kubernetes",
	Long:  `List, monitor, and clean up RECAC agent pods and jobs in a Kubernetes cluster.`,
}

var k8sListCmd = &cobra.Command{
	Use:   "list",
	Short: "List active agent pods",
	RunE:  runK8sList,
}

var k8sLogsCmd = &cobra.Command{
	Use:   "logs [pod_name|ticket_id]",
	Short: "Stream logs from an agent pod",
	Args:  cobra.ExactArgs(1),
	RunE:  runK8sLogs,
}

var k8sCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Delete completed or failed agent jobs",
	RunE:  runK8sClean,
}

func init() {
	rootCmd.AddCommand(k8sCmd)
	k8sCmd.PersistentFlags().StringVarP(&k8sNamespace, "namespace", "n", "default", "Kubernetes namespace")
	k8sCmd.AddCommand(k8sListCmd)
	k8sCmd.AddCommand(k8sLogsCmd)
	k8sCmd.AddCommand(k8sCleanCmd)
}

func runK8sList(cmd *cobra.Command, args []string) error {
	client, err := k8sNativeClientFactory()
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	pods, err := client.CoreV1().Pods(k8sNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=recac-agent",
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No active agent pods found.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tTICKET\tAGE")

	for _, pod := range pods.Items {
		ticket := pod.Labels["ticket"]
		age := time.Since(pod.CreationTimestamp.Time).Round(time.Second)
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", pod.Name, pod.Status.Phase, ticket, age)
	}
	w.Flush()
	return nil
}

func runK8sLogs(cmd *cobra.Command, args []string) error {
	target := args[0]
	client, err := k8sNativeClientFactory()
	if err != nil {
		return err
	}

	ctx := cmd.Context()

	// Determine if target is a pod name or ticket ID
	podName := target
	if !strings.HasPrefix(target, "recac-agent-") {
		// Assume it's a ticket ID, try to find the pod
		pods, err := client.CoreV1().Pods(k8sNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: fmt.Sprintf("app=recac-agent,ticket=%s", target),
		})
		if err != nil {
			return fmt.Errorf("failed to search for pod: %w", err)
		}
		if len(pods.Items) == 0 {
			return fmt.Errorf("no pod found for ticket %s", target)
		}
		// Pick the most recent one
		sort.Slice(pods.Items, func(i, j int) bool {
			return pods.Items[i].CreationTimestamp.After(pods.Items[j].CreationTimestamp.Time)
		})
		podName = pods.Items[0].Name
	}

	req := client.CoreV1().Pods(k8sNamespace).GetLogs(podName, &corev1.PodLogOptions{
		Container: "agent",
		Follow:    true,
	})

	stream, err := req.Stream(ctx)
	if err != nil {
		return fmt.Errorf("failed to open log stream: %w", err)
	}
	defer stream.Close()

	_, err = io.Copy(cmd.OutOrStdout(), stream)
	return err
}

func runK8sClean(cmd *cobra.Command, args []string) error {
	client, err := k8sNativeClientFactory()
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	jobs, err := client.BatchV1().Jobs(k8sNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "app=recac-agent",
	})
	if err != nil {
		return fmt.Errorf("failed to list jobs: %w", err)
	}

	cleaned := 0
	for _, job := range jobs.Items {
		finished := false
		if job.Status.Succeeded > 0 {
			finished = true
		} else if job.Status.Failed > 0 {
			finished = true
		}

		if finished {
			fmt.Fprintf(cmd.OutOrStdout(), "Deleting job %s...\n", job.Name)
			// Delete background
			policy := metav1.DeletePropagationBackground
			err := client.BatchV1().Jobs(k8sNamespace).Delete(ctx, job.Name, metav1.DeleteOptions{
				PropagationPolicy: &policy,
			})
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Failed to delete %s: %v\n", job.Name, err)
			} else {
				cleaned++
			}
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Cleaned %d jobs.\n", cleaned)
	return nil
}
