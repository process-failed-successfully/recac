package main

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func TestK8sList(t *testing.T) {
	// Mock Client
	client := fake.NewSimpleClientset(
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "recac-agent-123",
				Namespace: "default",
				Labels: map[string]string{
					"app":    "recac-agent",
					"ticket": "RD-123",
				},
				CreationTimestamp: metav1.NewTime(time.Now().Add(-1 * time.Hour)),
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
			},
		},
	)

	// Mock Factory
	origFactory := k8sNativeClientFactory
	k8sNativeClientFactory = func() (kubernetes.Interface, error) {
		return client, nil
	}
	defer func() { k8sNativeClientFactory = origFactory }()

	// Capture Output
	buf := new(bytes.Buffer)
	k8sListCmd.SetOut(buf)
	k8sListCmd.SetErr(buf)

	// Run
	err := runK8sList(k8sListCmd, nil)
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "recac-agent-123")
	assert.Contains(t, output, "Running")
	assert.Contains(t, output, "RD-123")
}

func TestK8sClean(t *testing.T) {
	// Mock Client with Jobs
	client := fake.NewSimpleClientset(
		&batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "job-success",
				Namespace: "default",
				Labels: map[string]string{
					"app": "recac-agent",
				},
			},
			Status: batchv1.JobStatus{
				Succeeded: 1,
			},
		},
		&batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "job-active",
				Namespace: "default",
				Labels: map[string]string{
					"app": "recac-agent",
				},
			},
			Status: batchv1.JobStatus{
				Active: 1,
			},
		},
	)

	// Mock Factory
	origFactory := k8sNativeClientFactory
	k8sNativeClientFactory = func() (kubernetes.Interface, error) {
		return client, nil
	}
	defer func() { k8sNativeClientFactory = origFactory }()

	// Capture Output
	buf := new(bytes.Buffer)
	k8sCleanCmd.SetOut(buf)

	// Run
	err := runK8sClean(k8sCleanCmd, nil)
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Deleting job job-success")
	assert.NotContains(t, output, "Deleting job job-active")
	assert.Contains(t, output, "Cleaned 1 jobs")

	// Verify Deletion
	actions := client.Actions()
	deleted := false
	for _, action := range actions {
		if action.GetVerb() == "delete" && action.GetResource().Resource == "jobs" {
			delAction := action.(interface{ GetName() string })
			if delAction.GetName() == "job-success" {
				deleted = true
			}
		}
	}
	assert.True(t, deleted, "Job should have been deleted")
}
