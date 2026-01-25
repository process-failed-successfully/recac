package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type StackInfo struct {
	Languages      map[string]int `json:"languages"`
	Frameworks     []string       `json:"frameworks"`
	Infrastructure []string       `json:"infrastructure"`
	Databases      []string       `json:"databases"`
	CI             []string       `json:"ci"`
}

var stackCmd = &cobra.Command{
	Use:   "stack [path]",
	Short: "Identify the technology stack of the project",
	Long: `Scans the project to identify languages, frameworks, infrastructure, and databases.
Can output a summary table, a JSON object, or a Mermaid component diagram.`,
	RunE: runStack,
}

func init() {
	rootCmd.AddCommand(stackCmd)
	stackCmd.Flags().Bool("json", false, "Output as JSON")
	stackCmd.Flags().Bool("mermaid", false, "Output as a Mermaid component diagram")
}

func runStack(cmd *cobra.Command, args []string) error {
	root := "."
	if len(args) > 0 {
		root = args[0]
	}

	info, err := analyzeStack(root)
	if err != nil {
		return err
	}

	isJSON, _ := cmd.Flags().GetBool("json")
	isMermaid, _ := cmd.Flags().GetBool("mermaid")

	if isJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(info)
	}

	if isMermaid {
		printStackMermaid(cmd, info)
		return nil
	}

	printStackTable(cmd, info)
	return nil
}

func analyzeStack(root string) (*StackInfo, error) {
	info := &StackInfo{
		Languages:      make(map[string]int),
		Frameworks:     make([]string, 0),
		Infrastructure: make([]string, 0),
		Databases:      make([]string, 0),
		CI:             make([]string, 0),
	}

	ignore := DefaultIgnoreMap()

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if ignore[d.Name()] {
				return filepath.SkipDir
			}
			// Check for specific directories
			if d.Name() == ".github" {
				info.CI = appendUnique(info.CI, "GitHub Actions")
			}
			return nil
		}

		filename := d.Name()
		ext := strings.ToLower(filepath.Ext(filename))

		// Languages
		lang := getLanguage(ext, filename)
		if lang != "" {
			info.Languages[lang]++
		}

		// Infrastructure
		if filename == "Dockerfile" || strings.HasSuffix(filename, ".dockerfile") {
			info.Infrastructure = appendUnique(info.Infrastructure, "Docker")
		}
		if filename == "docker-compose.yml" || filename == "docker-compose.yaml" {
			info.Infrastructure = appendUnique(info.Infrastructure, "Docker Compose")
			// Scan compose for DBs
			dbs := scanDockerCompose(path)
			for _, db := range dbs {
				info.Databases = appendUnique(info.Databases, db)
			}
		}
		if strings.HasSuffix(filename, ".tf") {
			info.Infrastructure = appendUnique(info.Infrastructure, "Terraform")
		}
		if filename == "Chart.yaml" {
			info.Infrastructure = appendUnique(info.Infrastructure, "Helm")
		}
		if filename == "kustomization.yaml" {
			info.Infrastructure = appendUnique(info.Infrastructure, "Kustomize")
		}

		// CI
		if filename == ".gitlab-ci.yml" {
			info.CI = appendUnique(info.CI, "GitLab CI")
		}
		if filename == "Jenkinsfile" {
			info.CI = appendUnique(info.CI, "Jenkins")
		}
		if filename == "azure-pipelines.yml" {
			info.CI = appendUnique(info.CI, "Azure Pipelines")
		}

		// Frameworks
		if filename == "go.mod" {
			fw := scanGoMod(path)
			for _, f := range fw {
				info.Frameworks = appendUnique(info.Frameworks, f)
			}
		}
		if filename == "package.json" {
			fw := scanPackageJson(path)
			for _, f := range fw {
				info.Frameworks = appendUnique(info.Frameworks, f)
			}
		}
		if filename == "requirements.txt" {
			fw := scanRequirementsTxt(path)
			for _, f := range fw {
				info.Frameworks = appendUnique(info.Frameworks, f)
			}
		}

		return nil
	})

	// Add generic K8s if yaml files look like k8s? Too noisy.
	// But if we saw Helm/Kustomize, we know K8s is there.
	if stackContains(info.Infrastructure, "Helm") || stackContains(info.Infrastructure, "Kustomize") {
		info.Infrastructure = appendUnique(info.Infrastructure, "Kubernetes")
	}

	return info, err
}

func getLanguage(ext, filename string) string {
	switch ext {
	case ".go":
		return "Go"
	case ".js", ".jsx":
		return "JavaScript"
	case ".ts", ".tsx":
		return "TypeScript"
	case ".py":
		return "Python"
	case ".java":
		return "Java"
	case ".rb":
		return "Ruby"
	case ".rs":
		return "Rust"
	case ".php":
		return "PHP"
	case ".html":
		return "HTML"
	case ".css":
		return "CSS"
	case ".sh":
		return "Shell"
	case ".sql":
		return "SQL"
	}
	if filename == "Makefile" {
		return "Make"
	}
	return ""
}

func scanGoMod(path string) []string {
	var frameworks []string
	lines, err := readLines(path)
	if err != nil {
		return nil
	}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Detect major Go frameworks
		if strings.Contains(line, "github.com/gin-gonic/gin") {
			frameworks = append(frameworks, "Gin")
		}
		if strings.Contains(line, "github.com/gofiber/fiber") {
			frameworks = append(frameworks, "Fiber")
		}
		if strings.Contains(line, "github.com/labstack/echo") {
			frameworks = append(frameworks, "Echo")
		}
		if strings.Contains(line, "github.com/spf13/cobra") {
			frameworks = append(frameworks, "Cobra")
		}
		if strings.Contains(line, "github.com/charmbracelet/bubbletea") {
			frameworks = append(frameworks, "Bubble Tea")
		}
		if strings.Contains(line, "gorm.io/gorm") {
			frameworks = append(frameworks, "GORM")
		}
		if strings.Contains(line, "entgo.io/ent") {
			frameworks = append(frameworks, "Ent")
		}
	}
	return frameworks
}

func scanPackageJson(path string) []string {
	// Simple grep for now, proper JSON parsing is better but this is a heuristic
	// We check for "name": pattern to avoid false positives
	var frameworks []string
	lines, err := readLines(path)
	if err != nil {
		return nil
	}
	for _, line := range lines {
		// Remove whitespace
		line = strings.TrimSpace(line)

		if strings.Contains(line, "\"react\":") {
			frameworks = appendUnique(frameworks, "React")
		}
		if strings.Contains(line, "\"vue\":") {
			frameworks = appendUnique(frameworks, "Vue")
		}
		if strings.Contains(line, "\"@angular/core\":") {
			frameworks = appendUnique(frameworks, "Angular")
		}
		if strings.Contains(line, "\"svelte\":") {
			frameworks = appendUnique(frameworks, "Svelte")
		}
		if strings.Contains(line, "\"next\":") {
			frameworks = appendUnique(frameworks, "Next.js")
		}
		if strings.Contains(line, "\"express\":") {
			frameworks = appendUnique(frameworks, "Express")
		}
		if strings.Contains(line, "\"nest.js\":") || strings.Contains(line, "\"@nestjs/core\":") {
			frameworks = appendUnique(frameworks, "NestJS")
		}
		if strings.Contains(line, "\"tailwindcss\":") {
			frameworks = appendUnique(frameworks, "Tailwind CSS")
		}
	}
	return frameworks
}

func scanRequirementsTxt(path string) []string {
	var frameworks []string
	lines, err := readLines(path)
	if err != nil {
		return nil
	}
	for _, line := range lines {
		line = strings.ToLower(line)
		if strings.Contains(line, "django") {
			frameworks = append(frameworks, "Django")
		}
		if strings.Contains(line, "flask") {
			frameworks = append(frameworks, "Flask")
		}
		if strings.Contains(line, "fastapi") {
			frameworks = append(frameworks, "FastAPI")
		}
		if strings.Contains(line, "pandas") {
			frameworks = append(frameworks, "Pandas")
		}
		if strings.Contains(line, "numpy") {
			frameworks = append(frameworks, "NumPy")
		}
		if strings.Contains(line, "torch") || strings.Contains(line, "pytorch") {
			frameworks = append(frameworks, "PyTorch")
		}
		if strings.Contains(line, "tensorflow") {
			frameworks = append(frameworks, "TensorFlow")
		}
	}
	return frameworks
}

func scanDockerCompose(path string) []string {
	var dbs []string
	lines, err := readLines(path)
	if err != nil {
		return nil
	}
	for _, line := range lines {
		// Look for images
		if strings.Contains(line, "image:") {
			if strings.Contains(line, "postgres") {
				dbs = appendUnique(dbs, "PostgreSQL")
			}
			if strings.Contains(line, "mysql") {
				dbs = appendUnique(dbs, "MySQL")
			}
			if strings.Contains(line, "mongo") {
				dbs = appendUnique(dbs, "MongoDB")
			}
			if strings.Contains(line, "redis") {
				dbs = appendUnique(dbs, "Redis")
			}
			if strings.Contains(line, "elasticsearch") {
				dbs = appendUnique(dbs, "Elasticsearch")
			}
			if strings.Contains(line, "mariadb") {
				dbs = appendUnique(dbs, "MariaDB")
			}
		}
	}
	return dbs
}

func appendUnique(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}

func stackContains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func printStackTable(cmd *cobra.Command, info *StackInfo) {
	fmt.Fprintln(cmd.OutOrStdout(), "📦 PROJECT STACK")
	fmt.Fprintln(cmd.OutOrStdout(), "================")

	// Top Languages
	fmt.Fprintln(cmd.OutOrStdout(), "\nLANGUAGES:")
	if len(info.Languages) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "  None detected")
	} else {
		// Sort by count
		type kv struct {
			Key   string
			Value int
		}
		var ss []kv
		for k, v := range info.Languages {
			ss = append(ss, kv{k, v})
		}
		sort.Slice(ss, func(i, j int) bool {
			return ss[i].Value > ss[j].Value
		})

		for _, kv := range ss {
			fmt.Fprintf(cmd.OutOrStdout(), "  - %s (%d files)\n", kv.Key, kv.Value)
		}
	}

	fmt.Fprintln(cmd.OutOrStdout(), "\nFRAMEWORKS & LIBRARIES:")
	if len(info.Frameworks) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "  None detected")
	} else {
		for _, f := range info.Frameworks {
			fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", f)
		}
	}

	fmt.Fprintln(cmd.OutOrStdout(), "\nINFRASTRUCTURE & CI:")
	allInfra := append(info.Infrastructure, info.CI...)
	if len(allInfra) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "  None detected")
	} else {
		for _, i := range allInfra {
			fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", i)
		}
	}

	fmt.Fprintln(cmd.OutOrStdout(), "\nDATABASES:")
	if len(info.Databases) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "  None detected")
	} else {
		for _, d := range info.Databases {
			fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", d)
		}
	}
}

func printStackMermaid(cmd *cobra.Command, info *StackInfo) {
	fmt.Fprintln(cmd.OutOrStdout(), "graph TD")

	// Application Node (represented by Top Language + Frameworks)
	appLabel := "Application"
	if len(info.Languages) > 0 {
		// Find top language
		topLang := ""
		max := 0
		for k, v := range info.Languages {
			if v > max {
				max = v
				topLang = k
			}
		}
		appLabel = topLang + " App"
	}

	fmt.Fprintf(cmd.OutOrStdout(), "    App[%s]\n", appLabel)

	// Connect Frameworks
	for i, f := range info.Frameworks {
		nodeID := fmt.Sprintf("FW%d", i)
		fmt.Fprintf(cmd.OutOrStdout(), "    %s([%s]) -.-> App\n", nodeID, f)
	}

	// Connect Infrastructure
	for i, infra := range info.Infrastructure {
		nodeID := fmt.Sprintf("Infra%d", i)
		fmt.Fprintf(cmd.OutOrStdout(), "    App --- %s[%s]\n", nodeID, infra)
	}

	// Connect CI
	for i, ci := range info.CI {
		nodeID := fmt.Sprintf("CI%d", i)
		fmt.Fprintf(cmd.OutOrStdout(), "    %s{{%s}} --> App\n", nodeID, ci)
	}

	// Connect Databases
	for i, db := range info.Databases {
		nodeID := fmt.Sprintf("DB%d", i)
		fmt.Fprintf(cmd.OutOrStdout(), "    App <--> %s[(%s)]\n", nodeID, db)
	}
}
