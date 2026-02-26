package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func generateSecurityFixes(ctx context.Context, filePath string, findings []SecurityResult) ([]ReviewIssue, error) {
	contentBytes, err := readFileFunc(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}
	content := string(contentBytes)

	// Construct a summary of findings
	var findingsDesc strings.Builder
	for _, f := range findings {
		findingsDesc.WriteString(fmt.Sprintf("- Line %d: [%s] %s (Match: %s)\n", f.Line, f.Type, f.Description, f.Match))
	}

	provider := viper.GetString("provider")
	model := viper.GetString("model")
	cwd, _ := os.Getwd()

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-security-fix")
	if err != nil {
		return nil, err
	}

	prompt := fmt.Sprintf(`You are a security expert fixing vulnerabilities in code.
I will provide a file and a list of security findings detected by a scanner.
Your job is to analyze each finding and determine if it is a true positive.
If it is a true positive, provide a safe replacement for the vulnerable code.

File: %s
Findings:
%s

File Content:
%s

Instructions:
1. Analyze each finding in the context of the file.
2. If a finding is a False Positive, ignore it.
3. If a finding is valid, provide a fix.
4. The "replacement" field should contain the COMPLETE replacement code for the lines you want to change.
5. The "original_content" field must contain the EXACT content of the lines you are replacing (for verification).
6. Return a JSON list of objects.

JSON Structure:
[
  {
    "line": <start_line_number>,
    "title": "Fix: <Finding Type>",
    "description": "Explanation of why this is an issue and how it is fixed.",
    "severity": "CRITICAL",
    "suggestion": "Description of the fix (shown in UI)",
    "replacement": "<new code>",
    "original_content": "<exact old code>"
  }
]

Do not include findings that are false positives.
Return ONLY the raw JSON.
`, filePath, findingsDesc.String(), content)

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return nil, err
	}

	cleanResp := utils.CleanJSONBlock(resp)
	var issues []ReviewIssue
	if err := json.Unmarshal([]byte(cleanResp), &issues); err != nil {
		// Log warning but return partial results? No, return error.
		return nil, fmt.Errorf("failed to parse AI response: %w", err)
	}

	// Post-process issues to set File path
	for i := range issues {
		issues[i].File = filePath
		if issues[i].Severity == "" {
			issues[i].Severity = "CRITICAL"
		}
	}

	return issues, nil
}

func runInteractiveSecurityFix(cmd *cobra.Command, findings []SecurityResult) error {
	// Group by file
	findingsByFile := make(map[string][]SecurityResult)
	for _, f := range findings {
		findingsByFile[f.File] = append(findingsByFile[f.File], f)
	}

	var allIssues []ReviewIssue

	fmt.Fprintf(cmd.OutOrStdout(), "🔍 Analyzing %d findings across %d files for fixes...\n", len(findings), len(findingsByFile))

	ctx := cmd.Context()
	for file, fileFindings := range findingsByFile {
		fmt.Fprintf(cmd.OutOrStdout(), "  • Analyzing %s...\n", file)
		issues, err := generateSecurityFixes(ctx, file, fileFindings)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "    ⚠️ Failed to generate fix for %s: %v\n", file, err)
			continue
		}
		if len(issues) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "    ✅ Found %d fixable issues\n", len(issues))
			allIssues = append(allIssues, issues...)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "    ⚪ No AI fixes suggested (might be false positives)\n")
		}
	}

	if len(allIssues) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No fixes generated. All findings might be false positives or too complex for auto-fix.")
		return nil
	}

	fmt.Fprintln(cmd.OutOrStdout(), "\n🚀 Starting interactive fix session...")
	return runReviewTUIFunc(initialReviewModel(allIssues))
}
