package architecture

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// MockFileSystem for testing
type MockFileSystem struct {
	Files map[string]os.FileInfo
}

func (m *MockFileSystem) Stat(name string) (os.FileInfo, error) {
	if info, ok := m.Files[name]; ok {
		return info, nil
	}
	return nil, os.ErrNotExist
}

// MockFileInfo implements os.FileInfo
type MockFileInfo struct {
	NameVal string
}

func (m MockFileInfo) Name() string       { return m.NameVal }
func (m MockFileInfo) Size() int64        { return 100 }
func (m MockFileInfo) Mode() os.FileMode  { return 0644 }
func (m MockFileInfo) ModTime() time.Time { return time.Now() }
func (m MockFileInfo) IsDir() bool        { return false }
func (m MockFileInfo) Sys() interface{}   { return nil }

func TestValidator_Validate(t *testing.T) {
	mockFS := &MockFileSystem{
		Files: map[string]os.FileInfo{
			"schema.json":   MockFileInfo{NameVal: "schema.json"},
			"contract.yaml": MockFileInfo{NameVal: "contract.yaml"},
		},
	}
	validator := NewValidator(mockFS)

	tests := []struct {
		name    string
		arch    SystemArchitecture
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid Architecture",
			arch: SystemArchitecture{
				Version:    "1.0",
				SystemName: "TestSys",
				Components: []Component{
					{
						ID:   "comp1",
						Type: "service",
						Contracts: []Contract{
							{Type: "openapi", Path: "contract.yaml"},
						},
						Produces: []Output{
							{Event: "EventA", Schema: "schema.json"},
						},
					},
					{
						ID:   "comp2",
						Type: "worker",
						Consumes: []Input{
							{Source: "comp1", Type: "EventA", Schema: "schema.json"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Missing Version",
			arch: SystemArchitecture{
				SystemName: "TestSys",
				Components: []Component{{ID: "c1", Type: "s"}},
			},
			wantErr: true,
			errMsg:  "version is required",
		},
		{
			name: "Missing System Name",
			arch: SystemArchitecture{
				Version:    "1.0",
				Components: []Component{{ID: "c1", Type: "s"}},
			},
			wantErr: true,
			errMsg:  "system_name is required",
		},
		{
			name: "No Components",
			arch: SystemArchitecture{
				Version:    "1.0",
				SystemName: "TestSys",
				Components: []Component{},
			},
			wantErr: true,
			errMsg:  "no components defined",
		},
		{
			name: "Duplicate Component ID",
			arch: SystemArchitecture{
				Version:    "1.0",
				SystemName: "TestSys",
				Components: []Component{
					{ID: "c1", Type: "s"},
					{ID: "c1", Type: "s"},
				},
			},
			wantErr: true,
			errMsg:  "duplicate component ID: c1",
		},
		{
			name: "Component Output Missing Type",
			arch: SystemArchitecture{
				Version:    "1.0",
				SystemName: "TestSys",
				Components: []Component{
					{
						ID:   "c1",
						Type: "s",
						Produces: []Output{
							{Schema: "schema.json"},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "component c1 output missing type/event",
		},
		{
			name: "Missing Component ID",
			arch: SystemArchitecture{
				Version:    "1.0",
				SystemName: "TestSys",
				Components: []Component{
					{Type: "s"},
				},
			},
			wantErr: true,
			errMsg:  "missing ID",
		},
		{
			name: "Missing Component Type",
			arch: SystemArchitecture{
				Version:    "1.0",
				SystemName: "TestSys",
				Components: []Component{
					{ID: "c1"},
				},
			},
			wantErr: true,
			errMsg:  "missing type",
		},
		{
			name: "Contract File Not Found",
			arch: SystemArchitecture{
				Version:    "1.0",
				SystemName: "TestSys",
				Components: []Component{
					{
						ID:   "c1",
						Type: "s",
						Contracts: []Contract{
							{Path: "missing.yaml"},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "contract file not found: missing.yaml",
		},
		{
			name: "Input Source Not Found",
			arch: SystemArchitecture{
				Version:    "1.0",
				SystemName: "TestSys",
				Components: []Component{
					{
						ID:   "c1",
						Type: "s",
						Consumes: []Input{
							{Source: "missing_comp"},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "input source 'missing_comp' does not exist",
		},
		{
			name: "Input Schema Not Found",
			arch: SystemArchitecture{
				Version:    "1.0",
				SystemName: "TestSys",
				Components: []Component{
					{
						ID:   "c1",
						Type: "s",
						Consumes: []Input{
							{Source: "comp1", Schema: "missing.json"}, // comp1 exists in arch logic? No, validate just checks if source exists in passed arch.
							// But here components are processed in order? No, collected first.
							// Wait, for this test case, we need comp1 to exist if we reference it, or test specifically for schema not found.
							// If source doesn't exist, it fails on source check first.
							// So we need valid source.
						},
					},
					{ID: "comp1", Type: "s"},
				},
			},
			wantErr: true,
			errMsg:  "input schema file not found: missing.json",
		},
		{
			name: "Output Target Not Found",
			arch: SystemArchitecture{
				Version:    "1.0",
				SystemName: "TestSys",
				Components: []Component{
					{
						ID:   "c1",
						Type: "s",
						Produces: []Output{
							{Type: "T", Target: "missing_comp"},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "output target 'missing_comp' does not exist",
		},
		{
			name: "Output Schema Not Found",
			arch: SystemArchitecture{
				Version:    "1.0",
				SystemName: "TestSys",
				Components: []Component{
					{
						ID:   "c1",
						Type: "s",
						Produces: []Output{
							{Type: "T", Schema: "missing.json"},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "output schema file not found: missing.json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.Validate(&tt.arch)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Validate() expected error, got nil")
				} else if tt.errMsg != "" {
					if !strings.Contains(err.Error(), tt.errMsg) {
						t.Errorf("Validate() error = %v, want error containing %v", err, tt.errMsg)
					}
				}
			} else {
				if err != nil {
					t.Errorf("Validate() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestNewValidator(t *testing.T) {
	v := NewValidator(nil)
	if v == nil {
		t.Error("NewValidator(nil) returned nil")
	}
	if _, ok := v.FS.(RealFileSystem); !ok {
		t.Error("NewValidator(nil) did not default to RealFileSystem")
	}
}

func TestRealFileSystem_Stat(t *testing.T) {
	fs := RealFileSystem{}

	// Create temp file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(tmpFile, []byte("content"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	info, err := fs.Stat(tmpFile)
	if err != nil {
		t.Errorf("Stat failed: %v", err)
	}
	if info != nil {
		if info.Name() != "test.txt" {
			t.Errorf("Expected name test.txt, got %s", info.Name())
		}
		if info.Size() != 7 {
			t.Errorf("Expected size 7, got %d", info.Size())
		}
	}

	// Test non-existent
	_, err = fs.Stat(filepath.Join(tmpDir, "missing"))
	if !os.IsNotExist(err) {
		t.Errorf("Expected NotExist error, got %v", err)
	}
}
