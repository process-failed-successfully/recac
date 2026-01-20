package scenarios

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type SQLParserScenario struct{}

func (s *SQLParserScenario) Name() string {
	return "sql-parser"
}

func (s *SQLParserScenario) Description() string {
	return "Build a SQL-to-JSON AST parser for complex SELECT queries."
}

func (s *SQLParserScenario) AppSpec(repoURL string) string {
	return fmt.Sprintf(`### ID:[SQL-PARSER] SQL-to-JSON AST Parser

Build a tool that parses SQL SELECT queries and produces a JSON representation of the Abstract Syntax Tree (AST).

IMPORTANT: The Title of the corresponding Epic/Story MUST start with "ID:[SQL-PARSER]". Do not strip this marker.

Requirements:
1. Support complex SELECT queries including:
   - Nested subqueries
   - JOINs (INNER, LEFT, RIGHT)
   - WHERE clauses with multiple conditions
   - GROUP BY and HAVING
2. Output Format: Produce a JSON file that accurately represents the structure of the input SQL.
3. Verification: The tool will be tested against various complex queries.

Use any language (Python or Go preferred).
Ensure you use a bash block to create the source files.

Repo: %s`, repoURL)
}

func (s *SQLParserScenario) Generate(uniqueID string, repoURL string) []TicketSpec {
	return []TicketSpec{
		{
			ID:      "SQL-PARSER",
			Summary: fmt.Sprintf("[%s] Build a SQL-to-JSON AST Parser", uniqueID),
			Desc: fmt.Sprintf(`Write a function/script that converts raw SQL SELECT queries into a structured JSON Abstract Syntax Tree (AST).

The parser MUST handle:
1. SELECT clause with multiple columns or *
2. FROM clause
3. WHERE clause with basic operators (=, >, <) and logical operators (AND, OR) with parentheses.

Example Input:
SELECT name, age FROM users WHERE age > 25 AND (city = 'NY' OR city = 'SF')

Example Output (Structure may vary, but must show nesting):
{
  "type": "select",
  "columns": ["name", "age"],
  "from": "users",
  "where": {
    "type": "and",
    "left": {"type": "operator", "op": ">", "field": "age", "value": 25},
    "right": {
      "type": "or",
      "left": {"type": "operator", "op": "=", "field": "city", "value": "NY"},
      "right": {"type": "operator", "op": "=", "field": "city", "value": "SF"}
    }
  }
}

Use any language (Python or Go preferred).
The script should accept the SQL string as a command line argument and print the JSON to stdout.
Ensure you use a bash block to create the source files.

Repo: %s`, repoURL),
			Type: "Task",
		},
	}
}

func (s *SQLParserScenario) Verify(repoPath string, ticketKeys map[string]string) error {
	// Fetch all remote branches first to ensure agent branches are available
	fetchCmd := exec.Command("git", "fetch", "--all")
	fetchCmd.Dir = repoPath
	if out, err := fetchCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to fetch branches: %v\nOutput: %s", err, out)
	}

	var branch string
	ticketKey, ok := ticketKeys["SQL-PARSER"]
	if !ok {
		fmt.Println("Warning: SQL-PARSER ticket key not found in map. Falling back to generic agent branch detection.")
		b, err := getAgentBranch(repoPath)
		if err != nil {
			return fmt.Errorf("failed to find any agent branch (fallback): %w", err)
		}
		branch = b
		fmt.Printf("Fallback: Found agent branch %s\n", branch)
	} else {
		b, err := getSpecificAgentBranch(repoPath, ticketKey)
		if err != nil {
			fmt.Printf("Warning: Specific branch for %s not found. Falling back to generic agent branch detection.\n", ticketKey)
			b, err = getAgentBranch(repoPath)
			if err != nil {
				return fmt.Errorf("branch for %s not found and fallback failed: %w", ticketKey, err)
			}
			fmt.Printf("Fallback: Found agent branch %s\n", b)
		}
		branch = b
	}

	// Checkout branch
	checkoutCmd := exec.Command("git", "checkout", branch)
	checkoutCmd.Dir = repoPath
	if out, err := checkoutCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to checkout %s: %v\nOutput: %s", branch, err, out)
	}

	testQuery := "SELECT name, age FROM users WHERE age > 25 AND (status = 'active' OR role = 'admin')"

	// Discover entry point - check for common file patterns
	var cmd *exec.Cmd
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	goFiles := []string{"main.go", "parser.go", "sql_parser.go"}
	pyFiles := []string{"main.py", "parser.py", "sql_parser.py"}

	// Try Go files first if we have a go.mod or any .go file
	for _, f := range goFiles {
		if _, err := os.Stat(filepath.Join(repoPath, f)); err == nil {
			cmd = exec.CommandContext(ctx, "go", "run", ".", testQuery)
			cmd.Dir = repoPath
			break
		}
	}

	// Fall back to Python files
	if cmd == nil {
		for _, f := range pyFiles {
			if _, err := os.Stat(filepath.Join(repoPath, f)); err == nil {
				cmd = exec.CommandContext(ctx, "python3", f, testQuery)
				cmd.Dir = repoPath
				break
			}
		}
	}

	if cmd == nil {
		// List files for debugging
		files, _ := os.ReadDir(repoPath)
		var fileList []string
		for _, f := range files {
			fileList = append(fileList, f.Name())
		}
		return fmt.Errorf("could not determine how to run the parser. Files in repo: %v", fileList)
	}

	// Use separate buffers for stdout/stderr to avoid parsing errors from debug output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("parser execution failed: %v\nStdout: %s\nStderr: %s", err, stdout.String(), stderr.String())
	}

	// Extract JSON from stdout - handle potential non-JSON output before/after
	outStr := strings.TrimSpace(stdout.String())
	jsonStart := strings.Index(outStr, "{")
	jsonEnd := strings.LastIndex(outStr, "}")
	if jsonStart == -1 || jsonEnd == -1 || jsonEnd < jsonStart {
		return fmt.Errorf("no valid JSON object found in output: %s", outStr)
	}
	jsonStr := outStr[jsonStart : jsonEnd+1]

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return fmt.Errorf("failed to parse JSON output: %v\nExtracted JSON: %s\nFull output: %s", err, jsonStr, outStr)
	}

	// Basic Structural Validation with flexible key matching
	typeVal := getStringKey(result, "type", "statement_type", "kind")
	if typeVal == "" || !strings.Contains(strings.ToLower(typeVal), "select") {
		return fmt.Errorf("expected type containing 'select', got %v", result["type"])
	}

	// Check columns - flexible key matching
	cols := getArrayKey(result, "columns", "fields", "select_list", "expressions")
	if len(cols) < 2 {
		return fmt.Errorf("failed to find expected columns in AST (need at least 2)")
	}

	// Check where clause nesting (Deep Check) - flexible key matching
	where := getMapKey(result, "where", "where_clause", "condition", "filter")
	if where == nil {
		return fmt.Errorf("where clause missing in AST")
	}

	// We expect an AND at the top level of the WHERE clause for our test query
	// but the exact keys might vary. We'll search for logical operators.
	if !hasLogicalType(where, "and") {
		return fmt.Errorf("expected top-level 'and' in where clause, got: %v", where)
	}

	fmt.Println("SQL Parser AST depth check passed!")
	return nil
}

// getStringKey checks multiple possible keys and returns the first non-empty string value found
func getStringKey(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

// getArrayKey checks multiple possible keys and returns the first array found
func getArrayKey(m map[string]interface{}, keys ...string) []interface{} {
	for _, k := range keys {
		if v, ok := m[k].([]interface{}); ok {
			return v
		}
	}
	return nil
}

// getMapKey checks multiple possible keys and returns the first map found
func getMapKey(m map[string]interface{}, keys ...string) map[string]interface{} {
	for _, k := range keys {
		if v, ok := m[k].(map[string]interface{}); ok {
			return v
		}
	}
	return nil
}

func hasLogicalType(node map[string]interface{}, logicalType string) bool {
	t, ok := node["type"].(string)
	if ok && strings.ToLower(t) == logicalType {
		return true
	}
	// Recursive search just in case
	for _, v := range node {
		if m, ok := v.(map[string]interface{}); ok {
			if hasLogicalType(m, logicalType) {
				return true
			}
		}
	}
	return false
}

func init() {
	Register(&SQLParserScenario{})
}
