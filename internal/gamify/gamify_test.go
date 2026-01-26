package gamify

import (
	"testing"
)

type MockGitClient struct {
	LogFunc func(directory string, args ...string) ([]string, error)
}

func (m *MockGitClient) Log(directory string, args ...string) ([]string, error) {
	if m.LogFunc != nil {
		return m.LogFunc(directory, args...)
	}
	return nil, nil
}

func TestAnalyzeRepo(t *testing.T) {
	mockClient := &MockGitClient{
		LogFunc: func(directory string, args ...string) ([]string, error) {
			// Simulate git log output
			return []string{
				"COMMIT|abc1234|Alice|2023-10-25 10:00:00 +0000|Initial commit",
				"10 0 main.go",
				"5 0 README.md",
				"",
				"COMMIT|def5678|Bob|2023-10-26 12:00:00 +0000|Fix bug in main",
				"2 2 main.go",
				"",
				"COMMIT|ghi9012|Alice|2023-10-27 14:00:00 +0000|Add tests",
				"20 0 main_test.go",
			}, nil
		},
	}

	lb, err := AnalyzeRepo(mockClient, ".")
	if err != nil {
		t.Fatalf("AnalyzeRepo failed: %v", err)
	}

	if len(lb.Players) != 2 {
		t.Errorf("Expected 2 players, got %d", len(lb.Players))
	}

	// Verify Alice
	var alice *Player
	for _, p := range lb.Players {
		if p.Name == "Alice" {
			alice = p
			break
		}
	}

	if alice == nil {
		t.Fatal("Alice not found")
	}

	// Alice Stats:
	// Commit 1: 10 XP + 5 (10 lines) + 5 (doc) = 20 XP
	// Commit 2: 10 XP + 2 (20 lines) + 10 (test) = 22 XP
	// Total: 42 XP
	// Expected lines added: 10+5+20 = 35
	// Expected Test Edits: 1
	// Expected Doc Edits: 1

	// My XP Logic in code:
	// linesXP = added (capped 100) / 10
	// Commit 1: 10/10 = 1 XP (main.go), 5/10 = 0 XP (README). Total 1 XP from lines?
	// Wait, the loop runs per line of numstat.
	// main.go: 10 added -> 1 XP.
	// README.md: 5 added -> 0 XP. +5 Doc Bonus.
	// Total Lines XP: 1.
	// Total Bonuses: 5.
	// Base Commit XP: 10.
	// Total Commit 1: 10 + 1 + 5 = 16.

	// Commit 2 (Alice - "Add tests"):
	// main_test.go: 20 added -> 2 XP. +10 Test Bonus.
	// Base Commit XP: 10.
	// Total Commit 2: 10 + 2 + 10 = 22.

	// Grand Total: 16 + 22 = 38.

	if alice.Commits != 2 {
		t.Errorf("Expected Alice commits 2, got %d", alice.Commits)
	}
	if alice.TestEdits != 1 {
		t.Errorf("Expected Alice test edits 1, got %d", alice.TestEdits)
	}
	if alice.DocEdits != 1 {
		t.Errorf("Expected Alice doc edits 1, got %d", alice.DocEdits)
	}
	if alice.LinesAdded != 35 {
		t.Errorf("Expected Alice lines added 35, got %d", alice.LinesAdded)
	}

	// Let's verify XP roughly matches expectations (allow for integer division nuances)
	if alice.XP != 38 {
		t.Errorf("Expected Alice XP 38, got %d", alice.XP)
	}

	// Verify Bob
	var bob *Player
	for _, p := range lb.Players {
		if p.Name == "Bob" {
			bob = p
			break
		}
	}

	// Bob Stats:
	// Commit 1: "Fix bug" -> +20 XP. Base +10 XP.
	// Lines: 2 added -> 0 XP.
	// Total: 30 XP.

	if bob.XP != 30 {
		t.Errorf("Expected Bob XP 30, got %d", bob.XP)
	}
	if bob.BugFixes != 1 {
		t.Errorf("Expected Bob bug fixes 1, got %d", bob.BugFixes)
	}
}

func TestBadges(t *testing.T) {
	p := &Player{
		Commits:   60,
		BugFixes:  6,
		DocEdits:  15,
		TestEdits: 12,
		XP:        1500,
	}

	awardBadges(p)

	expectedBadges := []string{"🏅 Marathoner", "🐛 Hunter", "📜 Scholar", "🧪 Scientist", "🧙 Wizard"}

	for _, expected := range expectedBadges {
		found := false
		for _, b := range p.Badges {
			if b == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected badge %s not found", expected)
		}
	}
}
