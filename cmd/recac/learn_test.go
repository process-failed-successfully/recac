package main

import (
	"context"
	"os"
	"testing"
	"recac/internal/agent"
	"recac/internal/learn"
	"github.com/spf13/viper"
)

type TestMockAgent struct {
	agent.MockAgent
}

func (m *TestMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	return `{
		"question": "What is the meaning of life?",
		"answer": "42",
		"options": ["41", "42", "43"],
		"explanation": "Because Douglas Adams said so."
	}`, nil
}

func TestLearnGenerateAndStats(t *testing.T) {
	// Setup Temp Dir
	tmp, err := os.MkdirTemp("", "learn-cmd-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	// Change CWD
	origWd, _ := os.Getwd()
	if err := os.Chdir(tmp); err != nil {
		t.Fatal(err)
	}
	defer func() { os.Chdir(origWd) }()

	// Create dummy go file
	err = os.WriteFile("dummy.go", []byte("package main\nfunc main() {}"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	files, _ := os.ReadDir(".")
	for _, f := range files {
		t.Logf("File in tmp: %s", f.Name())
	}

	// Mock Factory
	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &TestMockAgent{}, nil
	}
	defer func() { agentClientFactory = origFactory }()

	// Config
	viper.Set("provider", "mock")
	viper.Set("model", "mock-model")

	// 1. Run Generate
	cmd := learnGenCmd
	learnLimit = 1

	if err := cmd.RunE(cmd, []string{"."}); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// 2. Verify Store
	store := learn.NewStore(tmp)
	cards, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(cards) != 1 {
		t.Errorf("Expected 1 card, got %d", len(cards))
	}
	if cards[0].Answer != "42" {
		t.Errorf("Expected Answer '42', got '%s'", cards[0].Answer)
	}

	// 3. Run Stats
	if err := runLearnStats(learnStatsCmd, []string{}); err != nil {
		t.Errorf("Stats failed: %v", err)
	}
}
