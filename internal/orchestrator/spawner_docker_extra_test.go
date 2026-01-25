package orchestrator

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDockerSpawner_Spawn_SaveSessionInitialFailure(t *testing.T) {
	mockDocker := new(MockDockerClient)
	mockSM := new(MockSessionManager)
	mockGit := new(MockGitClient)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	spawner := NewDockerSpawner(logger, mockDocker, "test-image", "test-proj", nil, "", "", mockSM)
	spawner.GitClient = mockGit

	item := WorkItem{ID: "TICKET-FAIL-SAVE", RepoURL: "repo"}
	ctx := context.Background()

	// Expectations
	mockDocker.On("RunContainer", ctx, "test-image", mock.Anything, mock.Anything, mock.Anything, "").Return("container-123", nil)
	// Fail the first SaveSession
	mockSM.On("SaveSession", mock.Anything).Return(errors.New("db error"))
	// Expect cleanup
	mockDocker.On("StopContainer", mock.Anything, "container-123").Return(nil)

	err := spawner.Spawn(ctx, item)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to save session state")
	mockDocker.AssertExpectations(t)
	mockSM.AssertExpectations(t)
}

func TestDockerSpawner_Spawn_GoroutineFailures(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*MockDockerClient, *MockSessionManager, *MockGitClient, *MockPoller, chan struct{})
	}{
		{
			name: "LoadSession Failure (Exec Success)",
			setupMocks: func(md *MockDockerClient, msm *MockSessionManager, mg *MockGitClient, mp *MockPoller, done chan struct{}) {
				md.On("RunContainer", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "").Return("c1", nil)
				msm.On("SaveSession", mock.Anything).Return(nil).Once() // Initial save

				md.On("Exec", mock.Anything, "c1", mock.Anything).Return("output", nil)

				// Fail LoadSession and signal done
				msm.On("LoadSession", "TICKET-1").Run(func(args mock.Arguments) {
					close(done)
				}).Return(nil, errors.New("load error"))

				// Expect NO Poller update since exec was success
			},
		},
		{
			name: "LoadSession Failure (Exec Failure)",
			setupMocks: func(md *MockDockerClient, msm *MockSessionManager, mg *MockGitClient, mp *MockPoller, done chan struct{}) {
				md.On("RunContainer", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "").Return("c1b", nil)
				msm.On("SaveSession", mock.Anything).Return(nil).Once() // Initial save

				// Fail Exec
				md.On("Exec", mock.Anything, "c1b", mock.Anything).Return("output", errors.New("exec error"))

				// Fail LoadSession
				msm.On("LoadSession", "TICKET-1").Return(nil, errors.New("load error"))

				// Expect Poller update since exec failed and signal done
				mp.On("UpdateStatus", mock.Anything, mock.Anything, "Failed", mock.Anything).Run(func(args mock.Arguments) {
					close(done)
				}).Return(nil)
			},
		},
		{
			name: "Exec Error",
			setupMocks: func(md *MockDockerClient, msm *MockSessionManager, mg *MockGitClient, mp *MockPoller, done chan struct{}) {
				md.On("RunContainer", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "").Return("c2", nil)
				msm.On("SaveSession", mock.Anything).Return(nil).Once()

				// Fail Exec
				md.On("Exec", mock.Anything, "c2", mock.Anything).Return("partial output", errors.New("exit code 1"))

				msm.On("LoadSession", "TICKET-1").Return(&runner.SessionState{Status: "running"}, nil)

				// Poller update failure
				mp.On("UpdateStatus", mock.Anything, mock.Anything, "Failed", mock.Anything).Return(nil)

				// Expect final save with error status and signal done
				msm.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
					return s.Status == "error" && s.Error == "exit code 1"
				})).Run(func(args mock.Arguments) {
					close(done)
				}).Return(nil)

				// Git SHA might be called or not depending on where it failed, likely called
				mg.On("CurrentCommitSHA", mock.Anything).Return("sha", nil)
			},
		},
		{
			name: "Git SHA Failure",
			setupMocks: func(md *MockDockerClient, msm *MockSessionManager, mg *MockGitClient, mp *MockPoller, done chan struct{}) {
				md.On("RunContainer", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "").Return("c3", nil)
				msm.On("SaveSession", mock.Anything).Return(nil).Once()

				md.On("Exec", mock.Anything, "c3", mock.Anything).Return("ok", nil)
				msm.On("LoadSession", "TICKET-1").Return(&runner.SessionState{Status: "running"}, nil)

				// Fail Git SHA
				mg.On("CurrentCommitSHA", mock.Anything).Return("", errors.New("git error"))

				// Expect final save with completed status and signal done
				msm.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
					return s.Status == "completed"
				})).Run(func(args mock.Arguments) {
					close(done)
				}).Return(nil)
			},
		},
		{
			name: "Final Save Failure",
			setupMocks: func(md *MockDockerClient, msm *MockSessionManager, mg *MockGitClient, mp *MockPoller, done chan struct{}) {
				md.On("RunContainer", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, "").Return("c4", nil)
				msm.On("SaveSession", mock.Anything).Return(nil).Once()

				md.On("Exec", mock.Anything, "c4", mock.Anything).Return("ok", nil)
				msm.On("LoadSession", "TICKET-1").Return(&runner.SessionState{Status: "running"}, nil)
				mg.On("CurrentCommitSHA", mock.Anything).Return("sha", nil)

				// Fail final save and signal done
				msm.On("SaveSession", mock.MatchedBy(func(s *runner.SessionState) bool {
					return s.Status == "completed"
				})).Run(func(args mock.Arguments) {
					close(done)
				}).Return(errors.New("db error"))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mockDocker := new(MockDockerClient)
			mockSM := new(MockSessionManager)
			mockGit := new(MockGitClient)
			mockPoller := new(MockPoller)

			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			spawner := NewDockerSpawner(logger, mockDocker, "img", "proj", mockPoller, "", "", mockSM)
			spawner.GitClient = mockGit

			done := make(chan struct{})
			tc.setupMocks(mockDocker, mockSM, mockGit, mockPoller, done)

			item := WorkItem{ID: "TICKET-1", RepoURL: "repo"}
			err := spawner.Spawn(context.Background(), item)
			assert.NoError(t, err)

			// Wait for goroutine
			select {
			case <-done:
				// Success
			case <-time.After(2 * time.Second):
				t.Fatal("timeout waiting for goroutine completion")
			}

			mockDocker.AssertExpectations(t)
			mockSM.AssertExpectations(t)
			mockGit.AssertExpectations(t)
			mockPoller.AssertExpectations(t)
		})
	}
}
