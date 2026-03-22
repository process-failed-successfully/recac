package orchestrator

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"recac/internal/git"
	"recac/internal/runner"
	"regexp"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type K8sSpawner struct {
	Client            kubernetes.Interface
	Namespace         string
	Image             string
	Poller            Poller // To update status on completion
	AgentProvider     string
	AgentModel        string
	PullPolicy        corev1.PullPolicy
	Logger            *slog.Logger
	SessionManager    ISessionManager
	GitClient         IGitClient
	MaxIterations     int
	ManagerFrequency  int
	TaskMaxIterations int
}

func NewK8sSpawner(logger *slog.Logger, image string, namespace string, poller Poller, provider, model string, pullPolicy corev1.PullPolicy, sm ISessionManager, maxIterations, managerFrequency, taskMaxIterations int) (*K8sSpawner, error) {
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

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}

	if namespace == "" {
		namespace = "default"
		// Try to read namespace file if in cluster
		if data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
			namespace = strings.TrimSpace(string(data))
		}
	}

	return &K8sSpawner{
		Client:            clientset,
		Namespace:         namespace,
		Image:             image,
		Poller:            poller,
		AgentProvider:     provider,
		AgentModel:        model,
		PullPolicy:        pullPolicy,
		Logger:            logger,
		SessionManager:    sm,
		GitClient:         git.NewClient(),
		MaxIterations:     maxIterations,
		ManagerFrequency:  managerFrequency,
		TaskMaxIterations: taskMaxIterations,
	}, nil
}

func (s *K8sSpawner) Spawn(ctx context.Context, item WorkItem) error {
	s.Logger.Info("Spawning K8s Job",
		"item", item.ID,
		"namespace", s.Namespace,
		"inject_provider", s.AgentProvider,
		"inject_model", s.AgentModel,
	)

	// Clean ID for K8s name (lowercase, replace invalid chars)
	safeID := sanitizeK8sName(item.ID)
	jobName := fmt.Sprintf("recac-agent-%s", safeID)

	// Check if job already exists
	existingJob, err := s.Client.BatchV1().Jobs(s.Namespace).Get(ctx, jobName, metav1.GetOptions{})
	if err == nil {
		// Job exists
		if existingJob.Status.Failed > 0 {
			s.Logger.Info("Found failed job, deleting to retry", "name", jobName)
			// Delete background
			delPolicy := metav1.DeletePropagationBackground
			if err := s.Client.BatchV1().Jobs(s.Namespace).Delete(ctx, jobName, metav1.DeleteOptions{PropagationPolicy: &delPolicy}); err != nil {
				return fmt.Errorf("failed to delete failed job: %w", err)
			}
			// We can return here and let the next poll cycle create it, OR try to create immediate.
			// K8s deletion is async, so usually better to return and wait.
			// BUT, to be "atomic" we might want to wait?
			// Let's return and log, next tick will create it.
			return fmt.Errorf("cleaning up failed job %s, will retry next cycle", jobName)
		} else if existingJob.Status.Succeeded > 0 {
			s.Logger.Info("Job already succeeded", "name", jobName)
			return nil
		} else {
			// Active or undefined state
			s.Logger.Info("Job already exists and is active", "name", jobName)
			return nil
		}
	} else if !strings.Contains(err.Error(), "not found") {
		// Real error
		return fmt.Errorf("failed to check for existing job: %w", err)
	}

	// Define Job
	ttl := int32(3600)  // 1 Hour TTL
	backoff := int32(6) // Retries enabled (standard K8s default)
	// Spec says: "RestartPolicy: Never". "Orchestrator monitors...".

	// Construct Env Vars
	envMap := collectAgentEnvVars(item, s.AgentProvider, s.AgentModel)
	envMap["RECAC_HOST_WORKSPACE_PATH"] = "/workspace"
	var envVars []corev1.EnvVar
	for k, v := range envMap {
		envVars = append(envVars, corev1.EnvVar{Name: k, Value: v})
	}

	// Auth Handling:
	// Use Secret for sensitive data if available.
	// For now, we assume a secret "recac-agent-secrets" exists and we load all keys from it.
	// This avoids passing secrets in plain text.
	secretName := os.Getenv("RECAC_AGENT_SECRET_NAME")
	if secretName == "" {
		secretName = "recac-agent-secrets" // fallback
	}

	envFrom := []corev1.EnvFromSource{
		{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
				Optional:             boolPtr(true),
			},
		},
	}

	// Command:
	// git clone <URL> . && recac start --jira <ID>
	// We need to inject GitHub Token into URL if using https
	// BUT, we can't easily modify the URL inside the container without the secret available.
	// We can use a script wrapper.
	// `recac start` handles workspace setup?
	// No, spec says: "Initialization (The 'Clone' Step): ... initContainer or first step ... performs git clone".
	// Let's use an InitContainer for cloning?
	// Or just one script.
	// "git clone https://$GITHUB_TOKEN@github.com/... ."

	// We'll trust the Orchestrator passed a clone-able URL or we use env var injection in the shell command.
	// item.RepoURL is plain.
	// Command:
	agentCmd := []string{
		"recac-agent",
		"--jira", item.ID,
		"--project", item.ID,
		"--image", s.Image,
		"--path", "/workspace",
		"--detached=false",
		"--cleanup=false",
		"--verbose",
		"--allow-dirty",
		"--repo-url", item.RepoURL,
		"--max-iterations", fmt.Sprintf("%d", s.MaxIterations),
		"--manager-frequency", fmt.Sprintf("%d", s.ManagerFrequency),
		"--task-max-iterations", fmt.Sprintf("%d", s.TaskMaxIterations),
	}

	cmd := ConstructShellCommand(agentCmd)

	createdAt := time.Now().Format(time.RFC3339)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: jobName,
			Labels: map[string]string{
				"created-by": "recac-orchestrator",
				"work-item":  item.ID,
			},
			Annotations: map[string]string{
				"created-at": createdAt,
			},
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: &ttl,
			BackoffLimit:            &backoff,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":        "recac-agent",
						"ticket":     item.ID,
						"created-by": "recac-orchestrator",
						"work-item":  item.ID,
					},
					Annotations: map[string]string{
						"created-at": createdAt,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyNever,
					EnableServiceLinks: boolPtr(false),
					Containers: []corev1.Container{
						{
							Name:            "agent",
							Image:           s.Image,
							ImagePullPolicy: s.PullPolicy,
							Command:         cmd,
							Args:            nil,
							Env:             envVars,
							EnvFrom:         envFrom,
							WorkingDir:      "/workspace",
							VolumeMounts: []corev1.VolumeMount{
								{Name: "workspace", MountPath: "/workspace"},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "workspace",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}

	// 1. Initialize session
	session := &runner.SessionState{
		Name:           item.ID,
		Status:         "running",
		StartTime:      time.Now(),
		Command:        agentCmd,
		Workspace:      "/workspace",
		Type:           "orchestrated-k8s",
		AgentStateFile: ".agent_state.json",
	}

	if err := s.SessionManager.SaveSession(session); err != nil {
		s.Logger.Error("failed to save initial session state", "item", item.ID, "error", err)
		return fmt.Errorf("failed to save session: %w", err)
	}

	// 2. Create Job
	_, err = s.Client.BatchV1().Jobs(s.Namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}

	s.Logger.Info("Job created", "name", jobName)

	// 3. Wait for Job completion
	waitErr := s.waitForJob(ctx, jobName)

	if waitErr != nil && ctx.Err() == context.DeadlineExceeded {
		s.Logger.Warn("Job timeout exceeded, canceling K8s Job", "jobName", jobName)
		if cancelErr := s.Cancel(context.Background(), item.ID); cancelErr != nil {
			s.Logger.Error("failed to cancel timed out job", "jobName", jobName, "error", cancelErr)
		}
	}

	// 4. Update session
	finalSession, loadErr := s.SessionManager.LoadSession(item.ID)

	// Try to get logs
	var output string
	if waitErr != nil {
		logs, err := s.GetLogs(ctx, item.ID)
		if err == nil && logs != nil {
			buf := make([]byte, 4096)
			n, _ := logs.Read(buf)
			output = string(buf[:n])
			logs.Close()
		}
	}

	if loadErr != nil {
		s.Logger.Error("failed to reload session for update", "item", item.ID, "error", loadErr)
		if waitErr != nil && s.Poller != nil {
			_ = s.Poller.UpdateStatus(ctx, item, "Failed", fmt.Sprintf("Job failed:\n%s\nOutput:\n%s", waitErr, output))
		}
		finalSession = session // Fallback
	} else {
		finalSession.EndTime = time.Now()
		if waitErr != nil {
			finalSession.Status = "error"
			finalSession.Error = waitErr.Error()
			s.Logger.Error("Job failed", "name", jobName, "error", waitErr, "output", output)
			if s.Poller != nil {
				_ = s.Poller.UpdateStatus(ctx, item, "Failed", fmt.Sprintf("Job failed:\n%s\nOutput:\n%s", waitErr, output))
			}
		} else {
			finalSession.Status = "completed"
			s.Logger.Info("Job completed successfully", "name", jobName)
		}
	}
	// EndCommitSHA is not easily accessible from remote pod without extra tooling
	finalSession.EndCommitSHA = ""

	if err := s.SessionManager.SaveSession(finalSession); err != nil {
		s.Logger.Error("failed to save final session state", "item", item.ID, "error", err)
	}

	return waitErr
}

func (s *K8sSpawner) waitForJob(ctx context.Context, jobName string) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			job, err := s.Client.BatchV1().Jobs(s.Namespace).Get(ctx, jobName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get job status: %w", err)
			}

			if job.Status.Succeeded > 0 {
				return nil
			}
			if job.Status.Failed > 0 {
				return fmt.Errorf("job failed with %d failed pods", job.Status.Failed)
			}
		}
	}
}

func (s *K8sSpawner) Cancel(ctx context.Context, jobID string) error {
	s.Logger.Info("Canceling K8s Job", "job", jobID)
	safeID := sanitizeK8sName(jobID)
	jobName := fmt.Sprintf("recac-agent-%s", safeID)

	// Delete background to cleanup pods
	delPolicy := metav1.DeletePropagationBackground
	if err := s.Client.BatchV1().Jobs(s.Namespace).Delete(ctx, jobName, metav1.DeleteOptions{PropagationPolicy: &delPolicy}); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("job %s not found", jobName)
		}
		return fmt.Errorf("failed to delete job %s: %w", jobName, err)
	}

	return nil
}

func (s *K8sSpawner) GetLogs(ctx context.Context, jobID string) (io.ReadCloser, error) {
	// Find Pods by label
	// The label is "work-item=<jobID>" on the PodTemplate
	selector := fmt.Sprintf("work-item=%s", jobID)
	pods, err := s.Client.CoreV1().Pods(s.Namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no active pods found for job %s", jobID)
	}

	// Get logs from the first pod
	// Usually there is only one pod per job unless retrying
	podName := pods.Items[0].Name

	req := s.Client.CoreV1().Pods(s.Namespace).GetLogs(podName, &corev1.PodLogOptions{})
	podLogs, err := req.Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to open log stream: %w", err)
	}

	// Return the stream directly
	return podLogs, nil
}

func (s *K8sSpawner) Cleanup(ctx context.Context, item WorkItem) error {
	// Handled by TTLSecondsAfterFinished
	return nil
}

func (s *K8sSpawner) Ping(ctx context.Context) error {
	// Check K8s API connectivity
	_, err := s.Client.Discovery().ServerVersion()
	if err != nil {
		return fmt.Errorf("kubernetes cluster unreachable: %w", err)
	}
	return nil
}

func boolPtr(b bool) *bool {
	return &b
}

var k8sNameSanitizerRegex = regexp.MustCompile("[^a-z0-9]+")

func sanitizeK8sName(name string) string {
	// Lowercase and replace non-alphanumeric with -
	name = strings.ToLower(name)
	name = k8sNameSanitizerRegex.ReplaceAllString(name, "-")
	return strings.Trim(name, "-")
}
