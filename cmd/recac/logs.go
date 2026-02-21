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

		// Try local logs first
		logFile, err := sm.GetSessionLogs(sessionName)
		if err == nil {
			file, err := os.Open(logFile)
			if err == nil {
				defer file.Close()
				streamReader(cmd.OutOrStdout(), bufio.NewReader(file), follow, filter)
				return
			}
		}

		// Try K8s logs if not found locally
		if k8sClient != nil {
			// Find pod by label
			pods, err := k8sClient.ListPods(context.Background(), fmt.Sprintf("ticket=%s", sessionName))
			if err == nil && len(pods) > 0 {
				podName := pods[0].Name
				streamK8sLogs(cmd, k8sClient, podName, follow, filter)
				return
			}
		}

		fmt.Fprintf(cmd.ErrOrStderr(), "Error: session '%s' not found locally or in Kubernetes\n", sessionName)
		exit(1)
	},
}

func streamReader(out io.Writer, reader *bufio.Reader, follow bool, filter string) {
	processLine := func(line string) {
		if filter == "" || strings.Contains(line, filter) {
			fmt.Fprint(out, line)
		}
	}

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				if line != "" {
					processLine(line)
				}
				break
			}
			break
		}
		processLine(line)
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
			processLine(line)
		}
	}
}

func streamK8sLogs(cmd *cobra.Command, client IK8sClient, podName string, follow bool, filter string) {
	stream, err := client.GetPodLogs(context.Background(), podName, follow)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error getting K8s logs: %v\n", err)
		return
	}
	defer stream.Close()

	reader := bufio.NewReader(stream)
	processLine := func(line string) {
		if filter == "" || strings.Contains(line, filter) {
			fmt.Fprint(cmd.OutOrStdout(), line)
		}
	}

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				if line != "" {
					processLine(line)
				}
				break
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Error reading K8s logs: %v\n", err)
			break
		}
		processLine(line)
	}
}

func streamAllRunningSessions(cmd *cobra.Command, sm ISessionManager, k8sClient IK8sClient, follow bool, filter string) {
	logChan := make(chan string)
	var wg sync.WaitGroup

	// Local sessions
	sessions, err := sm.ListSessions()
	if err == nil {
		for _, s := range sessions {
			if s.Status == "running" {
				wg.Add(1)
				go func(s *runner.SessionState) {
					defer wg.Done()
					logFile, err := sm.GetSessionLogs(s.Name)
					if err != nil {
						return
					}
					file, err := os.Open(logFile)
					if err != nil {
						return
					}
					defer file.Close()

					reader := bufio.NewReader(file)
					streamToChan(reader, logChan, fmt.Sprintf("[%s] ", s.Name), follow)
				}(s)
			}
		}
	}

	// K8s pods
	if k8sClient != nil {
		// List pods with 'ticket' label
		pods, err := k8sClient.ListPods(context.Background(), "ticket")
		if err == nil {
			for _, pod := range pods {
				if pod.Status.Phase == "Running" {
					wg.Add(1)
					go func(podName, ticket string) {
						defer wg.Done()
						stream, err := k8sClient.GetPodLogs(context.Background(), podName, follow)
						if err != nil {
							return
						}
						defer stream.Close()

						reader := bufio.NewReader(stream)
						prefix := fmt.Sprintf("[%s] ", ticket)
						if ticket == "" {
							prefix = fmt.Sprintf("[%s] ", podName)
						}

						// For K8s stream, simple reading is enough as it blocks if follow=true
						for {
							line, err := reader.ReadString('\n')
							if err != nil {
								if line != "" {
									logChan <- prefix + line
								}
								break
							}
							logChan <- prefix + line
						}
					}(pod.Name, pod.Labels["ticket"])
				}
			}
		}
	}

	go func() {
		wg.Wait()
		close(logChan)
	}()

	foundAny := false
	for line := range logChan {
		foundAny = true
		if filter == "" || strings.Contains(line, filter) {
			fmt.Fprint(cmd.OutOrStdout(), line)
		}
	}

	if !foundAny {
		fmt.Fprintln(cmd.OutOrStdout(), "No running sessions found.")
	}
}

func streamToChan(reader *bufio.Reader, ch chan<- string, prefix string, follow bool) {
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				if line != "" {
					ch <- prefix + line
				}
				break
			}
			break
		}
		ch <- prefix + line
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
			ch <- prefix + line
		}
	}
}
