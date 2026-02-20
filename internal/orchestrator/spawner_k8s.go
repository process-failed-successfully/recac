package orchestrator

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type K8sSpawner struct {
	Client        kubernetes.Interface
	Namespace     string
	Image         string
	AgentProvider string
	AgentModel    string
	PullPolicy    corev1.PullPolicy
	Logger        *slog.Logger
}

func NewK8sSpawner(logger *slog.Logger, image string, namespace, provider, model string, pullPolicy corev1.PullPolicy) (*K8sSpawner, error) {
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
		Client:        clientset,
		Namespace:     namespace,
		Image:         image,
		AgentProvider: provider,
		AgentModel:    model,
		PullPolicy:    pullPolicy,
		Logger:        logger,
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
	args := fmt.Sprintf("recac-agent --jira %q --project %q --image %s --path /workspace --detached=false --cleanup=false --verbose --allow-dirty --repo-url %q", item.ID, item.ID, s.Image, item.RepoURL)
	if item.PlanOnly {
		args += " --plan"
	}

	cmd := fmt.Sprintf(`
		if [ -n "$GITHUB_TOKEN" ]; then
			git config --global url."https://${GITHUB_TOKEN}:x-oauth-basic@github.com/".insteadOf "https://github.com/"
		fi
		%s
	`, args)

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name: jobName,
			Labels: map[string]string{
				"created-by": "recac-orchestrator",
				"work-item":  item.ID,
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
				},
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyOnFailure,
					EnableServiceLinks: boolPtr(false),
					Containers: []corev1.Container{
						{
							Name:            "agent",
							Image:           s.Image,
							ImagePullPolicy: s.PullPolicy,
							Command:         []string{"/bin/sh", "-c"},
							Args:            []string{cmd},
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

	_, err = s.Client.BatchV1().Jobs(s.Namespace).Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}

	s.Logger.Info("Job created", "name", jobName)
	return nil
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
