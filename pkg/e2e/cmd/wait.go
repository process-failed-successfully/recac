package cmd

import (
	"flag"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"recac/pkg/e2e/state"
)

func RunWait(args []string) error {
	fs := flag.NewFlagSet("wait", flag.ExitOnError)
	var stateFile string
	fs.StringVar(&stateFile, "state-file", "e2e_state.json", "Path to state file")
	fs.Parse(args)

	e2eCtx, err := state.Load(stateFile)
	if err != nil {
		return fmt.Errorf("failed to load state file: %w", err)
	}

	namespace := e2eCtx.Namespace
	if namespace == "" {
		namespace = "default"
	}

	// 4. Wait for Execution
	log.Println("=== Waiting for Execution ===")
	// Check for Orchestrator Pod
	if err := waitForPod(namespace, "app.kubernetes.io/name=recac", 120*time.Second); err != nil {
		fmt.Println("!!! Orchestrator Pod Failed to Start !!!")
		printKubeDebugInfo(namespace)
		return fmt.Errorf("orchestrator pod failed to start: %w", err)
	}

	// Check for Agent Job
	log.Println("Waiting for Agent Job to start...")

	// Determine expected job name from ticket map (assuming single task for now or finding "PRIMES")
	var targetTicketID string
	if id, ok := e2eCtx.TicketMap["PRIMES"]; ok {
		targetTicketID = id
	} else {
		// Fallback: Use the first one
		for _, id := range e2eCtx.TicketMap {
			targetTicketID = id
			break
		}
	}
	
	if targetTicketID == "" {
		return fmt.Errorf("no ticket ID found in map")
	}

	expectedJobPrefix := fmt.Sprintf("recac-agent-%s", strings.ToLower(targetTicketID))
	log.Printf("Looking for job prefix: %s", expectedJobPrefix)

	jobName, err := waitForJob(namespace, expectedJobPrefix, 300*time.Second)
	if err != nil {
		printKubeDebugInfo(namespace)
		printLogs(namespace, "app.kubernetes.io/name=recac")
		return fmt.Errorf("agent job failed to start: %w", err)
	}
	log.Printf("Agent job started: %s", jobName)

	// Wait for Job Completion
	log.Println("Waiting for Agent Job to complete...")
	if err := waitForJobCompletion(namespace, jobName, 3600*time.Second); err != nil {
		printKubeDebugInfo(namespace)
		printLogs(namespace, "app=recac-agent")
		return fmt.Errorf("agent job failed to complete: %w", err)
	}

	// Print logs for debugging (especially for git push issues)
	cleanJobName := strings.TrimPrefix(jobName, "job.batch/")
	printLogs(namespace, fmt.Sprintf("job-name=%s", cleanJobName))

	return nil
}

func waitForPod(ns, labelSelector string, timeout time.Duration) error {
	return runCommand("kubectl", "rollout", "status", "deployment/recac", "-n", ns, "--timeout", fmt.Sprintf("%.0fs", timeout.Seconds()))
}

func waitForJob(ns, namePrefix string, timeout time.Duration) (string, error) {
	start := time.Now()
	for time.Since(start) < timeout {
		cmd := exec.Command("kubectl", "get", "jobs", "-n", ns, "-o", "name")
		out, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(out), "\n")
			for _, line := range lines {
				if strings.Contains(line, namePrefix) {
					return strings.TrimSpace(line), nil
				}
			}
		}
		time.Sleep(5 * time.Second)
	}
	return "", fmt.Errorf("timeout waiting for job %s", namePrefix)
}

func waitForJobCompletion(ns, jobName string, timeout time.Duration) error {
	start := time.Now()
	for time.Since(start) < timeout {
		// Check for success
		cmd := exec.Command("kubectl", "get", jobName, "-n", ns, "-o", "jsonpath={.status.succeeded}")
		out, _ := cmd.Output()
		if string(out) == "1" {
			return nil
		}

		// Check for failure
		cmdFail := exec.Command("kubectl", "get", jobName, "-n", ns, "-o", "jsonpath={.status.failed}")
		outFail, _ := cmdFail.Output()
		if string(outFail) != "" && string(outFail) != "0" {
			return fmt.Errorf("job failed with %s failures", string(outFail))
		}

		// Log status
		log.Printf("Waiting for job %s to complete... (%s elapsed)", jobName, time.Since(start).Round(time.Second))
		time.Sleep(10 * time.Second)
	}
	return fmt.Errorf("timeout waiting for job %s", jobName)
}

func printLogs(ns, selector string) {
	fmt.Println("--- LOGS START ---")
	_ = runCommand("kubectl", "logs", "-l", selector, "-n", ns, "--all-containers=true", "--tail=300")
	fmt.Println("--- LOGS END ---")
}

func printKubeDebugInfo(ns string) {
	fmt.Println("--- KUBE DEBUG INFO START ---")
	fmt.Println(">>> PODS <<<")
	_ = runCommand("kubectl", "get", "pods", "-n", ns, "-o", "wide")
	fmt.Println(">>> DESCRIBE PODS <<<")
	_ = runCommand("kubectl", "describe", "pods", "-n", ns)
	fmt.Println(">>> EVENTS <<<")
	_ = runCommand("kubectl", "get", "events", "-n", ns, "--sort-by=.lastTimestamp")
	fmt.Println("--- KUBE DEBUG INFO END ---")
}
