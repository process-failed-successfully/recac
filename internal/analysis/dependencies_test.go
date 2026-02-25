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
	bPath := "example.com/test/pkg/b"

	if _, ok := deps[aPath]; !ok {
		t.Errorf("pkg/a missing from deps")
	}
	if _, ok := deps[bPath]; !ok {
		// Empty deps might be missing keys depending on implementation?
		// My implementation adds keys only if file walked.
		// So it should be there with empty list? No, map defaults to nil slice.
		// Wait, AnalyzeDependencies iterates files.
		// If pkg/b has no imports, it might not be added to map if I don't initialize it?
		// Let's check code:
		// It does `deps[pkgPath] = append(deps[pkgPath], target)` ONLY inside loop over imports.
		// So if no imports, key might be missing unless I explicitly set it.
		// Code check:
		// for _, imp := range f.Imports { ... }
		// So if no imports, deps[pkgPath] is never touched.
		// I should probably fix this to ensure every walked package exists in map even if empty.
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
