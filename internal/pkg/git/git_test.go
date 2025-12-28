package git

import (
	"strings"
	"testing"
)

func TestValidateBranchName(t *testing.T) {
	tests := []struct {
		name    string
		branch  string
		wantErr bool
	}{
		// Valid cases
		{"Simple", "feature", false},
		{"WithHyphen", "feature-new", false},
		{"WithUnderscore", "feature_new", false},
		{"WithSlash", "feature/new", false},
		{"WithDot", "v1.0.0", false},
		{"Complex", "group/feature-name_123.v1", false},

		// Invalid cases - Security/Injection
		{"Semicolon", "feature;rm-rf", true},
		{"Space", "feature name", true},
		{"Backtick", "feature`whoami`", true},
		{"Dollar", "feature$(whoami)", true},
		{"StartWithDash", "-option", true},
		{"StartWithDash2", "--option", true},

		// Invalid cases - Git Specific
		{"DoubleDot", "feature..name", true},
		{"EndWithSlash", "feature/", true},
		{"EndWithDot", "feature.", true},
		{"ContainColon", "feature:name", true},
		{"ContainQuestion", "feature?name", true},
		{"ContainAsterisk", "feature*name", true},
		{"ContainBracket", "feature[name", true},
		{"ContainTilde", "feature~name", true},
		{"ContainCaret", "feature^name", true},
		{"ContainBackslash", "feature\\name", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBranchName(tt.branch)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateBranchName(%q) error = %v, wantErr %v", tt.branch, err, tt.wantErr)
			}
		})
	}
}

// TestCreateBranch_Validation ensures CreateBranch calls ValidateBranchName
// We can't easily mock exec.Command here without more complex logic,
// but we can check if it fails fast for invalid input before calling git.
func TestCreateBranch_Validation(t *testing.T) {
	// This should fail validation immediately, NOT run git command
	err := CreateBranch("-invalid")
	if err == nil {
		t.Error("CreateBranch should fail for invalid branch name")
	} else if !strings.Contains(err.Error(), "invalid branch name") {
		t.Errorf("Expected validation error, got: %v", err)
	}
}
