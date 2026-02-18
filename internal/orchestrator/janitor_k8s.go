package orchestrator

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// K8sJanitorClient adapts Kubernetes Client to JanitorClient interface.
type K8sJanitorClient struct {
	client    kubernetes.Interface
	namespace string
}

// NewK8sJanitorClient creates a new K8sJanitorClient.
func NewK8sJanitorClient(client kubernetes.Interface, namespace string) *K8sJanitorClient {
	return &K8sJanitorClient{
		client:    client,
		namespace: namespace,
	}
}

// ListCandidates returns K8s Jobs managed by the orchestrator.
func (k *K8sJanitorClient) ListCandidates(ctx context.Context) ([]Candidate, error) {
	// List Jobs with label selector
	jobs, err := k.client.BatchV1().Jobs(k.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: "created-by=recac-orchestrator",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}

	var candidates []Candidate
	for _, job := range jobs.Items {
		candidates = append(candidates, Candidate{
			ID:        job.Name,
			Name:      job.Name,
			WorkItem:  job.Labels["work-item"],
			CreatedAt: job.CreationTimestamp.Time,
			Labels:    job.Labels,
		})
	}
	return candidates, nil
}

// Remove deletes the Job (and its pods via propagation).
func (k *K8sJanitorClient) Remove(ctx context.Context, id string) error {
	propagationPolicy := metav1.DeletePropagationBackground
	return k.client.BatchV1().Jobs(k.namespace).Delete(ctx, id, metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	})
}
