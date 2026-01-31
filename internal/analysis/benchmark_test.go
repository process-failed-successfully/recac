package analysis

import (
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkGenerateCallGraph(b *testing.B) {
	wd, _ := os.Getwd()
	targetDir := filepath.Join(wd, "..")

    // Check if we are actually finding files
    cg, _ := GenerateCallGraph(targetDir)
    if len(cg.Nodes) == 0 {
        // Fallback to current directory if parent is empty
        targetDir = wd
        cg, _ = GenerateCallGraph(targetDir)
        if len(cg.Nodes) == 0 {
             b.Logf("Warning: No nodes found in %s", targetDir)
        }
    } else {
         b.Logf("Found %d nodes in %s", len(cg.Nodes), targetDir)
    }

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := GenerateCallGraph(targetDir)
		if err != nil {
			b.Fatalf("GenerateCallGraph failed: %v", err)
		}
	}
}
