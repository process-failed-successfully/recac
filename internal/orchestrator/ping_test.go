package orchestrator

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"k8s.io/client-go/kubernetes/fake"
)

func TestFilePoller_Ping(t *testing.T) {
	f, err := os.CreateTemp("", "work-*.json")
	assert.NoError(t, err)
	defer os.Remove(f.Name())

	p := NewFilePoller(f.Name())
	assert.NoError(t, p.Ping(context.Background()))

	pFail := NewFilePoller("non-existent-file.json")
	assert.Error(t, pFail.Ping(context.Background()))
}

func TestFileDirPoller_Ping(t *testing.T) {
	dir, err := os.MkdirTemp("", "work-dir-*")
	assert.NoError(t, err)
	defer os.RemoveAll(dir)

	p, err := NewFileDirPoller(dir)
	assert.NoError(t, err)
	assert.NoError(t, p.Ping(context.Background()))

	pManual := &FileDirPoller{watchDir: "non-existent-dir"}
	assert.Error(t, pManual.Ping(context.Background()))
}

func TestJiraPoller_Ping(t *testing.T) {
	mockClient := new(MockJiraClient)
	p := NewJiraPoller(mockClient, "")

	// Success
	mockClient.On("SearchIssues", mock.Anything, "created is not empty").Return([]map[string]interface{}{}, nil).Once()
	assert.NoError(t, p.Ping(context.Background()))

	// Failure
	mockClient.On("SearchIssues", mock.Anything, "created is not empty").Return([]map[string]interface{}{}, errors.New("fail")).Once()
	assert.Error(t, p.Ping(context.Background()))
}

func TestDockerSpawner_Ping(t *testing.T) {
	mockClient := new(MockDockerClient)
	s := &DockerSpawner{Client: mockClient}

	// Success
	mockClient.On("ListContainers", mock.Anything, mock.Anything).Return([]types.Container{}, nil).Once()
	assert.NoError(t, s.Ping(context.Background()))

	// Failure
	mockClient.On("ListContainers", mock.Anything, mock.Anything).Return([]types.Container{}, errors.New("fail")).Once()
	assert.Error(t, s.Ping(context.Background()))
}

func TestK8sSpawner_Ping(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	s := &K8sSpawner{Client: fakeClient}

	assert.NoError(t, s.Ping(context.Background()))
}
