package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestK8sJanitorClient_ListCandidates(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	janitorClient := NewK8sJanitorClient(clientset, "default")
	ctx := context.Background()

	// Create jobs
	now := time.Now()

	// Job 1: Managed by recac
	job1 := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "job-1",
			Namespace: "default",
			Labels: map[string]string{
				"created-by": "recac-orchestrator",
				"work-item":  "TASK-1",
			},
			CreationTimestamp: metav1.Time{Time: now.Add(-2 * time.Hour)},
		},
	}

	// Job 2: Not managed by recac
	job2 := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "job-2",
			Namespace: "default",
			Labels: map[string]string{
				"created-by": "manual",
			},
			CreationTimestamp: metav1.Time{Time: now.Add(-1 * time.Hour)},
		},
	}

	_, err := clientset.BatchV1().Jobs("default").Create(ctx, job1, metav1.CreateOptions{})
	assert.NoError(t, err)
	_, err = clientset.BatchV1().Jobs("default").Create(ctx, job2, metav1.CreateOptions{})
	assert.NoError(t, err)

	candidates, err := janitorClient.ListCandidates(ctx)
	assert.NoError(t, err)
	assert.Len(t, candidates, 1)
	assert.Equal(t, "job-1", candidates[0].ID)
	assert.Equal(t, "TASK-1", candidates[0].WorkItem)
}

func TestK8sJanitorClient_Remove(t *testing.T) {
	clientset := fake.NewSimpleClientset()
	janitorClient := NewK8sJanitorClient(clientset, "default")
	ctx := context.Background()

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "job-to-remove",
			Namespace: "default",
		},
	}
	_, err := clientset.BatchV1().Jobs("default").Create(ctx, job, metav1.CreateOptions{})
	assert.NoError(t, err)

	err = janitorClient.Remove(ctx, "job-to-remove")
	assert.NoError(t, err)

	// Verify deletion
	_, err = clientset.BatchV1().Jobs("default").Get(ctx, "job-to-remove", metav1.GetOptions{})
	assert.Error(t, err) // Should be not found
}
