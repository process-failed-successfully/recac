package main

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"recac/internal/utils"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	mockInterface string
	mockOutput    string
	mockFramework string
	mockTestify   bool
)

var mockCmd = &cobra.Command{
	Use:   "mock [file]",
	Short: "Generate a mock implementation for an interface",
	Long: `Parses a Go file to find interfaces and generates a mock implementation using AI.
It supports generating standard struct mocks or 'testify/mock' compatible mocks.
`,
	Args: cobra.ExactArgs(1),
	RunE: runMock,
}

func init() {
	rootCmd.AddCommand(mockCmd)
	mockCmd.Flags().StringVarP(&mockInterface, "interface", "i", "", "Name of the interface to mock")
	mockCmd.Flags().StringVarP(&mockOutput, "output", "o", "", "Output file path")
	mockCmd.Flags().BoolVar(&mockTestify, "testify", false, "Use testify/mock (deprecated, use --framework)")
	mockCmd.Flags().StringVarP(&mockFramework, "framework", "f", "auto", "Mocking framework (auto, std, testify)")
}

func runMock(cmd *cobra.Command, args []string) error {
	filePath := args[0]
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("invalid file path: %w", err)
	}

	// 1. Parse File to find interfaces
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, absPath, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	interfaces := make(map[string]*ast.InterfaceType)
	var interfaceNames []string

	ast.Inspect(node, func(n ast.Node) bool {
		ts, ok := n.(*ast.TypeSpec)
		if !ok {
			return true
		}
		if iface, ok := ts.Type.(*ast.InterfaceType); ok {
			name := ts.Name.Name
			interfaces[name] = iface
			interfaceNames = append(interfaceNames, name)
		}
		return false
	})

	if len(interfaceNames) == 0 {
		return fmt.Errorf("no interfaces found in %s", filePath)
	}

	// 2. Select Interface
	targetInterface := mockInterface
	if targetInterface == "" {
		if len(interfaceNames) == 1 {
			targetInterface = interfaceNames[0]
		} else {
			err := askOneFunc(&survey.Select{
				Message: "Select interface to mock:",
				Options: interfaceNames,
			}, &targetInterface)
			if err != nil {
				return err // User cancelled
			}
		}
	} else {
		if _, exists := interfaces[targetInterface]; !exists {
			return fmt.Errorf("interface '%s' not found in file", targetInterface)
		}
	}

	// 3. Extract Interface Definition (Source Code)
	// We read the file again to get the source string
	content, err := os.ReadFile(absPath)
	if err != nil {
		return err
	}

	ifaceNode := interfaces[targetInterface]
	// AST positions are byte offsets
	start := fset.Position(ifaceNode.Pos()).Offset
	end := fset.Position(ifaceNode.End()).Offset

	// We want the whole declaration including "type Name interface"
	// Find the TypeSpec that contains this InterfaceType
	var typeSpec *ast.TypeSpec
	ast.Inspect(node, func(n ast.Node) bool {
		if ts, ok := n.(*ast.TypeSpec); ok && ts.Type == ifaceNode {
			typeSpec = ts
			return false
		}
		return true
	})

	if typeSpec != nil {
		start = fset.Position(typeSpec.Pos()).Offset
		end = fset.Position(typeSpec.End()).Offset
	}

	// Validate bounds
	if start < 0 || end > len(content) || start >= end {
		return fmt.Errorf("failed to extract source code for interface")
	}

	interfaceSource := string(content[start:end])

	// Also get package name
	packageName := node.Name.Name

	// 4. Determine Framework
	framework := mockFramework
	if framework == "auto" {
		if mockTestify {
			framework = "testify"
		} else {
			// Check if testify is used in go.mod?
			// For simplicity, default to std unless explicitly requested or detected in args
			framework = "std"

			// Simple heuristic: check if testify is imported in the file?
			// Unlikely for the interface definition file.
			// Let's stick to std as default safe bet.
		}
	}

	// 5. Generate Mock
	ctx := context.Background()
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	cwd, _ := os.Getwd()

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-mock")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	prompt := fmt.Sprintf(`Generate a Go mock implementation for the following interface.
Interface Name: %s
Package: %s
Framework: %s

Requirements:
- If Framework is "testify", use "github.com/stretchr/testify/mock".
- If Framework is "std", create a struct with function fields (e.g. "GetUsersFunc func() ...") or a simple struct that records calls.
- Make it thread-safe if possible.
- Include all necessary imports.
- Return ONLY the code for the mock file (including package declaration).

Code:
'''go
%s
'''`, targetInterface, packageName, framework, interfaceSource)

	fmt.Fprintf(cmd.OutOrStdout(), "🤖 Generating mock for %s (%s)...\n", targetInterface, framework)

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	cleanedCode := utils.CleanCodeBlock(resp)

	// 6. Output
	output := mockOutput
	if output == "" {
		// Default: [interface]_mock.go in same dir
		dir := filepath.Dir(absPath)
		filename := fmt.Sprintf("%s_mock.go", strings.ToLower(targetInterface))
		output = filepath.Join(dir, filename)
	}

	if err := os.WriteFile(output, []byte(cleanedCode), 0644); err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Mock saved to %s\n", output)

	return nil
}
