package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	adrDir = filepath.Join("docs", "adr")
)

var adrCmd = &cobra.Command{
	Use:   "adr",
	Short: "Manage Architecture Decision Records (ADRs)",
	Long:  `Create, list, and generate Architecture Decision Records (ADRs).`,
}

var adrInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize ADR directory",
	RunE:  runAdrInit,
}

var adrNewCmd = &cobra.Command{
	Use:   "new [title]",
	Short: "Create a new ADR",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runAdrNew,
}

var adrListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all ADRs",
	RunE:  runAdrList,
}

var adrGenerateCmd = &cobra.Command{
	Use:   "generate [description]",
	Short: "Generate an ADR using AI",
	Long:  "Drafts an Architecture Decision Record based on a description and project context.",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runAdrGenerate,
}

func init() {
	rootCmd.AddCommand(adrCmd)
	adrCmd.AddCommand(adrInitCmd)
	adrCmd.AddCommand(adrNewCmd)
	adrCmd.AddCommand(adrListCmd)
	adrCmd.AddCommand(adrGenerateCmd)
}

func runAdrInit(cmd *cobra.Command, args []string) error {
	if _, err := os.Stat(adrDir); err == nil {
		fmt.Printf("ADR directory %s already exists\n", adrDir)
		return nil
	}

	if err := os.MkdirAll(adrDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", adrDir, err)
	}

	// Create meta-ADR
	metaTitle := "Record Architecture Decisions"
	content := formatAdrContent(0, metaTitle, "Accepted",
		"We need to record architectural decisions.",
		"We will use Architecture Decision Records (ADRs) to track key decisions.",
		"We will have a history of decisions and their context.")

	filename := filepath.Join(adrDir, "0000-record-architecture-decisions.md")
	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		return err
	}

	fmt.Printf("Initialized ADRs in %s\n", adrDir)
	return nil
}

func runAdrNew(cmd *cobra.Command, args []string) error {
	title := strings.Join(args, " ")

	// Ensure init
	if _, err := os.Stat(adrDir); os.IsNotExist(err) {
		return fmt.Errorf("ADR directory not initialized. Run 'recac adr init' first.")
	}

	nextNum, err := getNextAdrNumber()
	if err != nil {
		return err
	}

	filename := fmt.Sprintf("%04d-%s.md", nextNum, sanitizeTitle(title))
	path := filepath.Join(adrDir, filename)

	content := formatAdrContent(nextNum, title, "Proposed",
		"{Context: What is the issue that we're seeing that is motivating this decision or change?}",
		"{Decision: What is the change that we're proposing and/or doing?}",
		"{Consequences: What becomes easier or more difficult to do and any risks introduced?}")

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write ADR: %w", err)
	}

	fmt.Printf("Created ADR %s: %s\n", fmt.Sprintf("%04d", nextNum), path)
	return nil
}

func runAdrList(cmd *cobra.Command, args []string) error {
	if _, err := os.Stat(adrDir); os.IsNotExist(err) {
		return fmt.Errorf("ADR directory not found. Run 'recac adr init' first.")
	}

	files, err := os.ReadDir(adrDir)
	if err != nil {
		return err
	}

	var adrs []AdrMeta
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".md") {
			meta, err := parseAdrFile(filepath.Join(adrDir, f.Name()))
			if err != nil {
				// Warn but continue
				// fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to parse %s: %v\n", f.Name(), err)
				continue
			}
			adrs = append(adrs, meta)
		}
	}

	// Print table
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tTITLE\tSTATUS\tDATE")
	for _, adr := range adrs {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", adr.ID, adr.Title, adr.Status, adr.Date)
	}
	w.Flush()

	return nil
}

func runAdrGenerate(cmd *cobra.Command, args []string) error {
	description := strings.Join(args, " ")

	if _, err := os.Stat(adrDir); os.IsNotExist(err) {
		return fmt.Errorf("ADR directory not initialized. Run 'recac adr init' first.")
	}

	ctx := context.Background()
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	cwd, _ := os.Getwd()

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-adr")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	prompt := fmt.Sprintf(`Draft an Architecture Decision Record (ADR) based on the following description.
Description: "%s"

Format the output as Markdown with the following sections:
- Title (inferred from description)
- Status: Proposed
- Date: Today's date
- Context
- Decision
- Consequences

Do not include the ID number in the title (it will be assigned automatically).
Return ONLY the markdown content.`, description)

	fmt.Fprintln(cmd.OutOrStdout(), "🤖 Drafting ADR...")

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return err
	}

	content := utils.CleanCodeBlock(resp)

	// Extract title to create filename
	title := "generated-adr"
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "# ") {
			title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
			break
		}
	}

	nextNum, err := getNextAdrNumber()
	if err != nil {
		return err
	}

	// Prepend ID to title in content if not present (AI might just say "# Title")
	// Actually, let's just rewrite the header line
	// But parsing is safer.

	filename := fmt.Sprintf("%04d-%s.md", nextNum, sanitizeTitle(title))
	path := filepath.Join(adrDir, filename)

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return err
	}

	fmt.Printf("Generated ADR %s: %s\n", fmt.Sprintf("%04d", nextNum), path)
	return nil
}

// Helpers

type AdrMeta struct {
	ID     string
	Title  string
	Status string
	Date   string
	File   string
}

func formatAdrContent(number int, title, status, context, decision, consequences string) string {
	date := time.Now().Format("2006-01-02")
	return fmt.Sprintf(`# %d. %s

Date: %s
Status: %s

## Context
%s

## Decision
%s

## Consequences
%s
`, number, title, date, status, context, decision, consequences)
}

func getNextAdrNumber() (int, error) {
	files, err := os.ReadDir(adrDir)
	if err != nil {
		return 0, err
	}

	maxNum := -1
	for _, f := range files {
		name := f.Name()
		if len(name) >= 4 && strings.HasSuffix(name, ".md") {
			if num, err := strconv.Atoi(name[:4]); err == nil {
				if num > maxNum {
					maxNum = num
				}
			}
		}
	}
	return maxNum + 1, nil
}

func sanitizeTitle(title string) string {
	title = strings.ToLower(title)
	reg, _ := regexp.Compile("[^a-z0-9]+")
	return strings.Trim(reg.ReplaceAllString(title, "-"), "-")
}

func parseAdrFile(path string) (AdrMeta, error) {
	f, err := os.Open(path)
	if err != nil {
		return AdrMeta{}, err
	}
	defer f.Close()

	meta := AdrMeta{
		File: filepath.Base(path),
	}

	// Parse ID from filename
	if len(meta.File) >= 4 {
		meta.ID = meta.File[:4]
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "# ") && meta.Title == "" {
			// # 0001. Title or # Title
			content := strings.TrimPrefix(line, "# ")
			parts := strings.SplitN(content, ". ", 2)
			if len(parts) == 2 {
				meta.Title = parts[1]
			} else {
				meta.Title = content
			}
		} else if strings.HasPrefix(line, "Status:") {
			meta.Status = strings.TrimSpace(strings.TrimPrefix(line, "Status:"))
		} else if strings.HasPrefix(line, "Date:") {
			meta.Date = strings.TrimSpace(strings.TrimPrefix(line, "Date:"))
		}
	}

	return meta, nil
}
