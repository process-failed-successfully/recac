package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// Mock Agent
type architectMockAgent struct {
	response string
}

func (m *architectMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.response, nil
}

func (m *architectMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.response, nil
}

func TestGenerateArchitecture(t *testing.T) {
	jsonResp := `{
		"architecture.yaml": "components: []",
		"contracts/service.yaml": "api: v1"
	}`

	ag := &architectMockAgent{response: "Here is the JSON:\n```json\n" + jsonResp + "\n```"}

	files, err := generateArchitecture(context.Background(), ag, "spec")
	if err != nil {
		t.Fatalf("generateArchitecture failed: %v", err)
	}

	if len(files) != 2 {
		t.Errorf("Expected 2 files, got %d", len(files))
	}
	if files["architecture.yaml"] != "components: []" {
		t.Errorf("Unexpected content for architecture.yaml")
	}
}

func TestGenerateArchitecture_RawJSON(t *testing.T) {
	jsonResp := `{
		"architecture.yaml": "components: []"
	}`

	ag := &architectMockAgent{response: jsonResp}

	files, err := generateArchitecture(context.Background(), ag, "spec")
	if err != nil {
		t.Fatalf("generateArchitecture failed: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("Expected 1 file, got %d", len(files))
	}
}

func TestBasePathFS(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "test"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	fs := &BasePathFS{Base: tmp}
	info, err := fs.Stat("test")
	if err != nil {
		t.Error(err)
	}
	if info != nil && info.Name() != "test" {
		// Just verifying it doesn't error and returns info
	}
}
