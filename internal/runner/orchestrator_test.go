package runner

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// customMockDBStore for specific test returns
type customMockDBStore struct {
	MockDBStore
	getFeaturesFunc func(projectID string) (string, error)
	getSignalFunc func(projectID, key string) (string, error)
}

func (c *customMockDBStore) GetFeatures(projectID string) (string, error) {
	if c.getFeaturesFunc != nil {
		return c.getFeaturesFunc(projectID)
	}
	return c.MockDBStore.GetFeatures(projectID)
}

func (c *customMockDBStore) GetSignal(projectID, key string) (string, error) {
	if c.getSignalFunc != nil {
		return c.getSignalFunc(projectID, key)
	}
	return c.MockDBStore.GetSignal(projectID, key)
}

func TestOrchestrator_Run_RefreshGraphError(t *testing.T) {
	tests := []struct {
		name        string
		storeFunc   func(projectID string) (string, error)
		expectedErr string
	}{
		{
			name: "db error on GetFeatures",
			storeFunc: func(projectID string) (string, error) {
				return "", errors.New("db error")
			},
			expectedErr: "db error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := &customMockDBStore{
				getFeaturesFunc: tt.storeFunc,
			}

			o := &Orchestrator{
				Project:   "test_project",
				DB:        store,
				Pool:      NewWorkerPool(1),
				Graph:     NewTaskGraph(),
				MaxAgents: 2,
			}

			err := o.Run(context.Background())
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedErr)
		})
	}
}

func TestOrchestrator_hasLifecycleSignal(t *testing.T) {
	tests := []struct {
		name       string
		signalFunc func(p, k string) (string, error)
		expected   bool
	}{
		{
			name: "no signal",
			signalFunc: func(p, k string) (string, error) {
				return "", nil
			},
			expected: false,
		},
		{
			name: "QA_PASSED signal",
			signalFunc: func(p, k string) (string, error) {
				if k == "QA_PASSED" {
					return "true", nil
				}
				return "", nil
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &Orchestrator{
				Project: "test_project",
				DB: &customMockDBStore{
					getSignalFunc: tt.signalFunc,
				},
				Pool:      NewWorkerPool(1),
				Graph:     NewTaskGraph(),
				MaxAgents: 1,
			}

			assert.Equal(t, tt.expected, o.hasLifecycleSignal())
		})
	}
}

func TestOrchestrator_GetMaxAgents(t *testing.T) {
	tests := []struct {
		name          string
		initialAgents int
		setAgents     *int
		expected      int
	}{
		{
			name:          "initial value",
			initialAgents: 42,
			expected:      42,
		},
		{
			name:          "after set value",
			initialAgents: 42,
			setAgents:     ptr(100),
			expected:      100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &Orchestrator{MaxAgents: tt.initialAgents}
			if tt.setAgents != nil {
				o.SetMaxAgents(*tt.setAgents)
			}
			assert.Equal(t, tt.expected, o.GetMaxAgents())
		})
	}
}

func TestOrchestrator_hasFailures(t *testing.T) {
	tests := []struct {
		name     string
		tasks    map[string]*TaskNode
		expected bool
	}{
		{
			name:     "no tasks",
			tasks:    map[string]*TaskNode{},
			expected: false,
		},
		{
			name: "has failed task",
			tasks: map[string]*TaskNode{
				"t1": {ID: "t1", Status: TaskFailed},
			},
			expected: true,
		},
		{
			name: "no failed task",
			tasks: map[string]*TaskNode{
				"t1": {ID: "t1", Status: TaskDone},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := &Orchestrator{
				Graph: NewTaskGraph(),
			}
			for k, v := range tt.tasks {
				o.Graph.Nodes[k] = v
			}
			assert.Equal(t, tt.expected, o.hasFailures())
		})
	}
}

func ptr[T any](v T) *T {
	return &v
}
