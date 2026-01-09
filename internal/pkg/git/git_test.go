package git

import (
	"testing"
)

// MockGitOps is a mock implementation of GitOps.
type MockGitOps struct {
	PushCalled          bool
	CreatePRCalled      bool
	CreateBranchCalled  bool
	ListBranchesCalled  bool
	GetLastCommitCalled bool
	LastBranch          string
	Branches            []string
	Commit              *Commit
	ListBranchesFunc    func(prefix string) ([]string, error)
}

func (m *MockGitOps) Push(branch string) error {
	m.PushCalled = true
	m.LastBranch = branch
	return nil
}

func (m *MockGitOps) CreatePR(branch, title, body string) error {
	m.CreatePRCalled = true
	m.LastBranch = branch
	return nil
}

func (m *MockGitOps) CreateBranch(name string) error {
	m.CreateBranchCalled = true
	m.LastBranch = name
	return nil
}

func (m *MockGitOps) ListBranches(prefix string) ([]string, error) {
	m.ListBranchesCalled = true
	if m.ListBranchesFunc != nil {
		return m.ListBranchesFunc(prefix)
	}
	return m.Branches, nil
}

func (m *MockGitOps) GetLastCommit(branch string) (*Commit, error) {
	m.GetLastCommitCalled = true
	m.LastBranch = branch
	return m.Commit, nil
}

func TestGitOps(t *testing.T) {
	mock := &MockGitOps{}
	oldDefault := DefaultGitOps
	DefaultGitOps = mock
	defer func() { DefaultGitOps = oldDefault }()

	t.Run("CreateBranch", func(t *testing.T) {
		err := CreateBranch("test-branch")
		if err != nil {
			t.Errorf("CreateBranch failed: %v", err)
		}
		if !mock.CreateBranchCalled {
			t.Error("CreateBranch was not called on mock")
		}
		if mock.LastBranch != "test-branch" {
			t.Errorf("Expected branch test-branch, got %s", mock.LastBranch)
		}
	})

	t.Run("Push", func(t *testing.T) {
		err := Push("test-branch")
		if err != nil {
			t.Errorf("Push failed: %v", err)
		}
		if !mock.PushCalled {
			t.Error("Push was not called on mock")
		}
	})

	t.Run("CreatePR", func(t *testing.T) {
		err := CreatePR("test-branch", "Title", "Body")
		if err != nil {
			t.Errorf("CreatePR failed: %v", err)
		}
		if !mock.CreatePRCalled {
			t.Error("CreatePR was not called on mock")
		}
	})
}
