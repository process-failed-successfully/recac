package analysis

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestAnalyzeDependencies(t *testing.T) {
	// 1. Setup Temp Dir
	tmpDir, err := os.MkdirTemp("", "recac-deps-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// 2. Create go.mod
	goMod := "module example.com/test\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	// 3. Create pkg/a/a.go (imports pkg/b and fmt)
	if err := os.MkdirAll(filepath.Join(tmpDir, "pkg", "a"), 0755); err != nil {
		t.Fatal(err)
	}
	aGo := `package a
import (
	"fmt"
	"example.com/test/pkg/b"
)
func A() {
	fmt.Println(b.B())
}`
	if err := os.WriteFile(filepath.Join(tmpDir, "pkg", "a", "a.go"), []byte(aGo), 0644); err != nil {
		t.Fatal(err)
	}

	// 4. Create pkg/b/b.go
	if err := os.MkdirAll(filepath.Join(tmpDir, "pkg", "b"), 0755); err != nil {
		t.Fatal(err)
	}
	bGo := `package b
func B() string { return "B" }`
	if err := os.WriteFile(filepath.Join(tmpDir, "pkg", "b", "b.go"), []byte(bGo), 0644); err != nil {
		t.Fatal(err)
	}

	// 5. Run Analysis
	opts := DependencyOptions{
		Root:       tmpDir,
		ModuleName: "example.com/test",
		ShowStdLib: true,
	}
	deps, err := AnalyzeDependencies(opts)
	if err != nil {
		t.Fatalf("AnalyzeDependencies failed: %v", err)
	}

	// 6. Assertions
	// Expected:
	// example.com/test/pkg/a -> [fmt, example.com/test/pkg/b]
	// example.com/test/pkg/b -> []

	aPath := "example.com/test/pkg/a"

	if _, ok := deps[aPath]; !ok {
		t.Errorf("pkg/a missing from deps")
	}

	// Check pkg/a imports
	imports := deps[aPath]
	sort.Strings(imports)
	expected := []string{"example.com/test/pkg/b", "fmt"}

	if len(imports) != len(expected) {
		t.Errorf("pkg/a imports mismatch. Got %v, want %v", imports, expected)
	} else {
		for i, v := range imports {
			if v != expected[i] {
				t.Errorf("pkg/a import[%d] = %s, want %s", i, v, expected[i])
			}
		}
	}
}

func TestAnalyzeDependencies_Ignore(t *testing.T) {
	// Setup similar structure but ignore pkg/b
	tmpDir, err := os.MkdirTemp("", "recac-deps-ignore-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	goMod := "module example.com/test\n\ngo 1.21\n"
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644)
	os.MkdirAll(filepath.Join(tmpDir, "pkg", "a"), 0755)

	aGo := `package a
import "example.com/test/pkg/ignoreme"
`
	os.WriteFile(filepath.Join(tmpDir, "pkg", "a", "a.go"), []byte(aGo), 0644)

	opts := DependencyOptions{
		Root:           tmpDir,
		ModuleName:     "example.com/test",
		IgnorePatterns: []string{"ignoreme"},
	}

	deps, err := AnalyzeDependencies(opts)
	if err != nil {
		t.Fatal(err)
	}

	// Should not contain ignoreme in imports
	imports := deps["example.com/test/pkg/a"]
	if len(imports) != 0 {
		t.Errorf("Expected 0 imports, got %v", imports)
	}
}

func TestGetModuleName(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "recac-deps-modname-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Case 1: Valid go.mod
	goMod := "module example.com/my-module\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}

	modName, err := GetModuleName(tmpDir)
	if err != nil {
		t.Errorf("GetModuleName failed: %v", err)
	}
	if modName != "example.com/my-module" {
		t.Errorf("Expected module name 'example.com/my-module', got '%s'", modName)
	}

	// Case 2: Missing go.mod
	missingDir, _ := os.MkdirTemp("", "recac-deps-missing-mod")
	defer os.RemoveAll(missingDir)

	_, err = GetModuleName(missingDir)
	if err == nil {
		t.Error("Expected error for missing go.mod, got nil")
	}

	// Case 3: Invalid go.mod (no module directive)
	invalidDir, _ := os.MkdirTemp("", "recac-deps-invalid-mod")
	defer os.RemoveAll(invalidDir)

	os.WriteFile(filepath.Join(invalidDir, "go.mod"), []byte("package main\n"), 0644)
	_, err = GetModuleName(invalidDir)
	if err == nil {
		t.Error("Expected error for invalid go.mod, got nil")
	}
}

func TestAnalyzeDependencies_AutoModule(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "recac-deps-auto-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	goMod := "module auto.com/mod\n\ngo 1.21\n"
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644)

	// Create a simple file
	os.MkdirAll(filepath.Join(tmpDir, "main"), 0755)
	mainGo := "package main\nimport \"fmt\"\nfunc main() { fmt.Println() }"
	os.WriteFile(filepath.Join(tmpDir, "main", "main.go"), []byte(mainGo), 0644)

	opts := DependencyOptions{
		Root: tmpDir,
		// No ModuleName provided
		ShowStdLib: true,
	}

	deps, err := AnalyzeDependencies(opts)
	if err != nil {
		t.Fatalf("AnalyzeDependencies failed: %v", err)
	}

	// Check if dependencies were analyzed correctly (at least check if module name was resolved implicitly)
	// If module name was resolved, we should see auto.com/mod/main in keys

	expectedPkg := "auto.com/mod/main"
	if _, ok := deps[expectedPkg]; !ok {
		t.Errorf("Expected package %s not found in deps. Found keys: %v", expectedPkg, deps)
	}
}
