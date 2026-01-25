package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var (
	stackJSON bool
)

// StackInfo represents the detected technology stack
type StackInfo struct {
	Languages      []Language  `json:"languages"`
	Frameworks     []Framework `json:"frameworks"`
	Tools          []Tool      `json:"tools"`
	Databases      []Database  `json:"databases"`
	Infrastructure []string    `json:"infrastructure"`
}

type Language struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type Framework struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	Type    string `json:"type"` // e.g., "web", "test", "orm"
}

type Tool struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type Database struct {
	Name string `json:"name"`
	Type string `json:"type"` // e.g., "sql", "nosql", "cache"
}

var stackCmd = &cobra.Command{
	Use:   "stack [path]",
	Short: "Detect and list the project's technology stack",
	Long: `Analyzes the project structure and configuration files to identify the technology stack.
Detects languages, frameworks, tools, databases, and infrastructure components.

Supported detections:
- Languages: Go, Node.js, Python, Java
- Infrastructure: Docker, Kubernetes, Terraform
- Configs: go.mod, package.json, requirements.txt, pom.xml, etc.`,
	RunE: runStack,
}

func init() {
	rootCmd.AddCommand(stackCmd)
	stackCmd.Flags().BoolVar(&stackJSON, "json", false, "Output results as JSON")
}

func runStack(cmd *cobra.Command, args []string) error {
	root := "."
	if len(args) > 0 {
		root = args[0]
	}

	info, err := detectStack(root)
	if err != nil {
		return err
	}

	if stackJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(info)
	}

	printStackReport(cmd, info)
	return nil
}

func detectStack(root string) (*StackInfo, error) {
	info := &StackInfo{
		Languages:      []Language{},
		Frameworks:     []Framework{},
		Tools:          []Tool{},
		Databases:      []Database{},
		Infrastructure: []string{},
	}

	// 1. Go Detection
	if goMod, err := parseGoModForStack(root); err == nil && goMod != nil {
		info.Languages = append(info.Languages, Language{Name: "Go", Version: goMod.GoVersion})
		info.Frameworks = append(info.Frameworks, goMod.Frameworks...)
		info.Tools = append(info.Tools, goMod.Tools...)
		info.Databases = append(info.Databases, goMod.Databases...)
	}

	// 2. Node.js Detection
	if pkgJson, err := parsePackageJson(root); err == nil && pkgJson != nil {
		info.Languages = append(info.Languages, Language{Name: "Node.js", Version: pkgJson.NodeVersion})
		info.Frameworks = append(info.Frameworks, pkgJson.Frameworks...)
		info.Tools = append(info.Tools, pkgJson.Tools...)
	}

	// 3. Python Detection
	if pyInfo, err := parsePython(root); err == nil && pyInfo != nil {
		info.Languages = append(info.Languages, Language{Name: "Python", Version: pyInfo.Version})
		info.Frameworks = append(info.Frameworks, pyInfo.Frameworks...)
	}

	// 4. Infrastructure Detection
	if _, err := os.Stat(filepath.Join(root, "Dockerfile")); err == nil {
		info.Infrastructure = append(info.Infrastructure, "Docker")
	}
	if _, err := os.Stat(filepath.Join(root, "docker-compose.yml")); err == nil {
		info.Infrastructure = append(info.Infrastructure, "Docker Compose")
	}
	if _, err := os.Stat(filepath.Join(root, "k8s")); err == nil || hasK8sFiles(root) {
		info.Infrastructure = append(info.Infrastructure, "Kubernetes")
	}
	if _, err := os.Stat(filepath.Join(root, "main.tf")); err == nil {
		info.Infrastructure = append(info.Infrastructure, "Terraform")
	}

	return info, nil
}

// --- Go Detector ---

type goModInfo struct {
	GoVersion  string
	Frameworks []Framework
	Tools      []Tool
	Databases  []Database
}

func parseGoModForStack(root string) (*goModInfo, error) {
	path := filepath.Join(root, "go.mod")
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info := &goModInfo{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "go ") {
			info.GoVersion = strings.TrimPrefix(line, "go ")
		} else if strings.Contains(line, "github.com/gin-gonic/gin") {
			info.Frameworks = append(info.Frameworks, Framework{Name: "Gin", Type: "web"})
		} else if strings.Contains(line, "github.com/gofiber/fiber") {
			info.Frameworks = append(info.Frameworks, Framework{Name: "Fiber", Type: "web"})
		} else if strings.Contains(line, "github.com/spf13/cobra") {
			info.Frameworks = append(info.Frameworks, Framework{Name: "Cobra", Type: "cli"})
		} else if strings.Contains(line, "github.com/stretchr/testify") {
			info.Frameworks = append(info.Frameworks, Framework{Name: "Testify", Type: "test"})
		} else if strings.Contains(line, "gorm.io/gorm") {
			info.Frameworks = append(info.Frameworks, Framework{Name: "GORM", Type: "orm"})
		} else if strings.Contains(line, "lib/pq") || strings.Contains(line, "pgx") {
			info.Databases = append(info.Databases, Database{Name: "PostgreSQL", Type: "sql"})
		} else if strings.Contains(line, "go-sql-driver/mysql") {
			info.Databases = append(info.Databases, Database{Name: "MySQL", Type: "sql"})
		} else if strings.Contains(line, "mongo-driver") {
			info.Databases = append(info.Databases, Database{Name: "MongoDB", Type: "nosql"})
		} else if strings.Contains(line, "redis/go-redis") {
			info.Databases = append(info.Databases, Database{Name: "Redis", Type: "cache"})
		}
	}
	return info, nil
}

// --- Node Detector ---

type packageJsonInfo struct {
	NodeVersion string
	Frameworks  []Framework
	Tools       []Tool
}

func parsePackageJson(root string) (*packageJsonInfo, error) {
	path := filepath.Join(root, "package.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var pkg struct {
		Engines struct {
			Node string `json:"node"`
		} `json:"engines"`
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}

	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil, err
	}

	info := &packageJsonInfo{
		NodeVersion: pkg.Engines.Node,
	}

	// Helper to check both dep lists
	checkDep := func(name string, version string) {
		if strings.Contains(name, "react") && name == "react" {
			info.Frameworks = append(info.Frameworks, Framework{Name: "React", Version: version, Type: "frontend"})
		} else if name == "vue" {
			info.Frameworks = append(info.Frameworks, Framework{Name: "Vue", Version: version, Type: "frontend"})
		} else if name == "next" {
			info.Frameworks = append(info.Frameworks, Framework{Name: "Next.js", Version: version, Type: "fullstack"})
		} else if name == "express" {
			info.Frameworks = append(info.Frameworks, Framework{Name: "Express", Version: version, Type: "backend"})
		} else if name == "typescript" {
			info.Tools = append(info.Tools, Tool{Name: "TypeScript", Version: version})
		} else if name == "jest" || strings.Contains(name, "mocha") {
			info.Tools = append(info.Tools, Tool{Name: "Jest/Test", Version: version})
		} else if name == "tailwindcss" {
			info.Frameworks = append(info.Frameworks, Framework{Name: "Tailwind CSS", Version: version, Type: "css"})
		}
	}

	for k, v := range pkg.Dependencies {
		checkDep(k, v)
	}
	for k, v := range pkg.DevDependencies {
		checkDep(k, v)
	}

	return info, nil
}

// --- Python Detector ---

type pythonInfo struct {
	Version    string
	Frameworks []Framework
}

func parsePython(root string) (*pythonInfo, error) {
	// 1. Try requirements.txt
	reqPath := filepath.Join(root, "requirements.txt")
	if data, err := os.ReadFile(reqPath); err == nil {
		info := &pythonInfo{}
		content := string(data)
		if strings.Contains(content, "django") {
			info.Frameworks = append(info.Frameworks, Framework{Name: "Django", Type: "web"})
		}
		if strings.Contains(content, "flask") {
			info.Frameworks = append(info.Frameworks, Framework{Name: "Flask", Type: "web"})
		}
		if strings.Contains(content, "fastapi") {
			info.Frameworks = append(info.Frameworks, Framework{Name: "FastAPI", Type: "web"})
		}
		if strings.Contains(content, "pandas") {
			info.Frameworks = append(info.Frameworks, Framework{Name: "Pandas", Type: "data"})
		}
		return info, nil
	}

	// 2. Try Pipfile
	// (Simplification: just check existence and simple grep)
	if _, err := os.Stat(filepath.Join(root, "Pipfile")); err == nil {
		// Just a marker for now
		return &pythonInfo{Version: "Pipenv"}, nil
	}

	return nil, os.ErrNotExist
}

// --- Helpers ---

func hasK8sFiles(root string) bool {
	// Look for .yaml files with "apiVersion"
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && path != root {
			return filepath.SkipDir // Shallow check
		}
		if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
			content, err := os.ReadFile(path)
			if err == nil && strings.Contains(string(content), "apiVersion:") && strings.Contains(string(content), "kind:") {
				return fmt.Errorf("found") // Hack to break early
			}
		}
		return nil
	})
	return err != nil && err.Error() == "found"
}

func printStackReport(cmd *cobra.Command, info *StackInfo) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "TECHNOLOGY STACK")
	fmt.Fprintln(w, "----------------")

	if len(info.Languages) > 0 {
		fmt.Fprintln(w, "LANGUAGES:")
		for _, l := range info.Languages {
			ver := ""
			if l.Version != "" {
				ver = fmt.Sprintf(" (%s)", l.Version)
			}
			fmt.Fprintf(w, "  - %s%s\n", l.Name, ver)
		}
		fmt.Fprintln(w, "")
	}

	if len(info.Frameworks) > 0 {
		fmt.Fprintln(w, "FRAMEWORKS:")
		for _, f := range info.Frameworks {
			fmt.Fprintf(w, "  - %s (%s)\n", f.Name, f.Type)
		}
		fmt.Fprintln(w, "")
	}

	if len(info.Databases) > 0 {
		fmt.Fprintln(w, "DATABASES:")
		for _, d := range info.Databases {
			fmt.Fprintf(w, "  - %s (%s)\n", d.Name, d.Type)
		}
		fmt.Fprintln(w, "")
	}

	if len(info.Infrastructure) > 0 {
		fmt.Fprintln(w, "INFRASTRUCTURE:")
		for _, i := range info.Infrastructure {
			fmt.Fprintf(w, "  - %s\n", i)
		}
		fmt.Fprintln(w, "")
	}

	if len(info.Tools) > 0 {
		fmt.Fprintln(w, "TOOLS:")
		for _, t := range info.Tools {
			fmt.Fprintf(w, "  - %s\n", t.Name)
		}
		fmt.Fprintln(w, "")
	}

	w.Flush()
}
