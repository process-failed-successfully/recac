package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"recac/internal/runner"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
)

func init() {
	logsCmd.Flags().BoolP("follow", "f", false, "Follow log output (like tail -f)")
	logsCmd.Flags().String("filter", "", "Filter logs by string match")
	logsCmd.Flags().Bool("all", false, "Stream logs from all running sessions")
	rootCmd.AddCommand(logsCmd)
}

var logsCmd = &cobra.Command{
	Use:   "logs [session-name]",
	Short: "View session logs",
	Long:  `View logs for a specific session or stream logs from all running sessions.`,
	Args: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")
		if all && len(args) > 0 {
			return fmt.Errorf("cannot use session name with --all")
		}
		if !all && len(args) != 1 {
			return fmt.Errorf("requires a session name or --all flag")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		all, _ := cmd.Flags().GetBool("all")
		follow := cmd.Flags().Lookup("follow").Changed
		filter, _ := cmd.Flags().GetString("filter")

		sm, err := sessionManagerFactory()
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: failed to create session manager: %v\n", err)
			exit(1)
		}

		k8sClient, _ := k8sClientFactory()

		if all {
			streamAllRunningSessions(cmd, sm, k8sClient, follow, filter)
			return
		}

		sessionName := args[0]

		// Try local session first
		logFile, err := sm.GetSessionLogs(sessionName)
		if err == nil {
			file, err := os.Open(logFile)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Error: failed to open log file: %v\n", err)
				exit(1)
			}
			defer file.Close()

			reader := bufio.NewReader(file)

			// Helper to process line
			processLine := func(line string) {
				if filter == "" || strings.Contains(line, filter) {
					fmt.Fprint(cmd.OutOrStdout(), line)
				}
			}

			// Initial read
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					if err == io.EOF {
						if line != "" {
							processLine(line)
						}
						break
					}
					fmt.Fprintf(cmd.ErrOrStderr(), "Error reading log file: %v\n", err)
					exit(1)
				}
				processLine(line)
			}

			if follow {
				// Follow mode
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						if err == io.EOF {
							time.Sleep(500 * time.Millisecond)
							continue
						}
						fmt.Fprintf(cmd.ErrOrStderr(), "Error streaming logs: %v\n", err)
						break
					}
					processLine(line)
				}
			}
			return
		}

		// Try K8s if local failed
		if k8sClient != nil {
			// First try to find pod by label 'ticket=sessionName'
			pods, err := k8sClient.ListPods(context.Background(), fmt.Sprintf("ticket=%s", sessionName))
			var podName string
			if err == nil && len(pods) > 0 {
				podName = pods[0].Name
			} else {
				// Fallback: assume sessionName is the podName
				podName = sessionName
			}

			opts := &corev1.PodLogOptions{
				Follow: follow,
			}
			stream, err := k8sClient.GetPodLogs(context.Background(), podName, opts)
			if err == nil {
				defer stream.Close()
				reader := bufio.NewReader(stream)
				for {
					line, err := reader.ReadString('\n')
					if err != nil {
						if err == io.EOF {
							if line != "" && (filter == "" || strings.Contains(line, filter)) {
								fmt.Fprint(cmd.OutOrStdout(), line)
							}
							break
						}
						// If streaming closed expectedly
						break
					}
					if filter == "" || strings.Contains(line, filter) {
						fmt.Fprint(cmd.OutOrStdout(), line)
					}
				}
				return
			}
		}

		fmt.Fprintf(cmd.ErrOrStderr(), "Error: session '%s' not found locally or in k8s.\n", sessionName)
		exit(1)
	},
}

func streamAllRunningSessions(cmd *cobra.Command, sm ISessionManager, k8sClient IK8sClient, follow bool, filter string) {
	var wg sync.WaitGroup
	logChan := make(chan string)
	foundAny := false

	// Local sessions
	sessions, err := sm.ListSessions()
	if err == nil {
		for _, s := range sessions {
			if s.Status == "running" {
				foundAny = true
				wg.Add(1)
				go func(s *runner.SessionState) {
					defer wg.Done()
					logFile, err := sm.GetSessionLogs(s.Name)
					if err != nil {
						logChan <- fmt.Sprintf("[%s] Error: %v\n", s.Name, err)
						return
					}
					file, err := os.Open(logFile)
					if err != nil {
						logChan <- fmt.Sprintf("[%s] Error: failed to open log file: %v\n", s.Name, err)
						return
					}
					defer file.Close()

					reader := bufio.NewReader(file)
					for {
						line, err := reader.ReadString('\n')
						if err != nil {
							if err == io.EOF && line != "" {
								logChan <- fmt.Sprintf("[%s] %s", s.Name, line)
							}
							break
						}
						logChan <- fmt.Sprintf("[%s] %s", s.Name, line)
					}

					if follow {
						for {
							line, err := reader.ReadString('\n')
							if err != nil {
								if err == io.EOF {
									time.Sleep(500 * time.Millisecond)
									continue
								}
								break
							}
							logChan <- fmt.Sprintf("[%s] %s", s.Name, line)
						}
					}
				}(s)
			}
		}
	}

	// Remote pods
	if k8sClient != nil {
		pods, err := k8sClient.ListPods(context.Background(), "app=recac-agent")
		if err == nil {
			for _, pod := range pods {
				if pod.Status.Phase == corev1.PodRunning {
					foundAny = true
					wg.Add(1)
					go func(pod corev1.Pod) {
						defer wg.Done()
						podName := pod.Name
						displayName := pod.Labels["ticket"]
						if displayName == "" {
							displayName = podName
						}

						opts := &corev1.PodLogOptions{Follow: follow}
						stream, err := k8sClient.GetPodLogs(context.Background(), podName, opts)
						if err != nil {
							return
						}
						defer stream.Close()

						reader := bufio.NewReader(stream)
						for {
							line, err := reader.ReadString('\n')
							if err != nil {
								if err == io.EOF && line != "" {
									logChan <- fmt.Sprintf("[%s] %s", displayName, line)
								}
								break
							}
							logChan <- fmt.Sprintf("[%s] %s", displayName, line)
						}
					}(pod)
				}
			}
		}
	}

	if !foundAny {
		fmt.Fprintln(cmd.OutOrStdout(), "No running sessions found locally or in K8s.")
		return
	}

	go func() {
		wg.Wait()
		close(logChan)
	}()

	for line := range logChan {
		if filter == "" || strings.Contains(line, filter) {
			fmt.Fprint(cmd.OutOrStdout(), line)
		}
	}
}
