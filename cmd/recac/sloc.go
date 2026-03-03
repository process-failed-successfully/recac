package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"recac/internal/utils"

	"github.com/spf13/cobra"
)

type slocStats struct {
	TotalFiles   int
	TotalLines   int
	BlankLines   int
	CommentLines int
	CodeLines    int
}

var slocCmd = &cobra.Command{
	Use:   "sloc [path]",
	Short: "Count Source Lines of Code",
	Long:  `Analyzes the specified directory (or current directory) and provides a statistical breakdown of Source Lines of Code (SLOC), including total lines, blank lines, and comments.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		root := "."
		if len(args) > 0 {
			root = args[0]
		}

		ignoreMap := DefaultIgnoreMap()
		extStats := make(map[string]*slocStats)
		var total slocStats

		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d.IsDir() {
				if ignoreMap[d.Name()] {
					return filepath.SkipDir
				}
				return nil
			}

			if utils.IsBinaryExt(path) {
				return nil
			}

			ext := strings.ToLower(filepath.Ext(path))
			if ext == "" {
				ext = filepath.Base(path)
			}

			stats, err := analyzeFileSLOC(path)
			if err != nil {
				// Log and continue if a specific file fails
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to analyze %s: %v\n", path, err)
				return nil
			}

			if _, ok := extStats[ext]; !ok {
				extStats[ext] = &slocStats{}
			}

			extStats[ext].TotalFiles++
			extStats[ext].TotalLines += stats.TotalLines
			extStats[ext].BlankLines += stats.BlankLines
			extStats[ext].CommentLines += stats.CommentLines
			extStats[ext].CodeLines += stats.CodeLines

			total.TotalFiles++
			total.TotalLines += stats.TotalLines
			total.BlankLines += stats.BlankLines
			total.CommentLines += stats.CommentLines
			total.CodeLines += stats.CodeLines

			return nil
		})

		if err != nil {
			return fmt.Errorf("failed to analyze sloc: %w", err)
		}

		// Prepare for sorting by code lines descending
		type extEntry struct {
			ext   string
			stats *slocStats
		}
		var entries []extEntry
		for ext, stats := range extStats {
			entries = append(entries, extEntry{ext, stats})
		}
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].stats.CodeLines > entries[j].stats.CodeLines
		})

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "EXTENSION\tFILES\tTOTAL\tBLANK\tCOMMENT\tCODE")
		fmt.Fprintln(w, "---------\t-----\t-----\t-----\t-------\t----")

		for _, entry := range entries {
			stats := entry.stats
			fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\t%d\n",
				entry.ext, stats.TotalFiles, stats.TotalLines, stats.BlankLines, stats.CommentLines, stats.CodeLines)
		}

		fmt.Fprintln(w, "---------\t-----\t-----\t-----\t-------\t----")
		fmt.Fprintf(w, "TOTAL\t%d\t%d\t%d\t%d\t%d\n",
			total.TotalFiles, total.TotalLines, total.BlankLines, total.CommentLines, total.CodeLines)

		w.Flush()

		return nil
	},
}

func analyzeFileSLOC(path string) (slocStats, error) {
	var stats slocStats

	file, err := os.Open(path)
	if err != nil {
		return stats, err
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(path))

	var commentPrefixes []string
	switch ext {
	case ".go", ".c", ".cpp", ".js", ".ts", ".java", ".cs", ".php", ".swift", ".rs":
		commentPrefixes = []string{"//", "/*", "*"}
	case ".py", ".rb", ".sh", ".bash", ".yaml", ".yml", ".tf":
		commentPrefixes = []string{"#"}
	case ".html", ".xml", ".svg":
		commentPrefixes = []string{"<!--"}
	case ".sql", ".lua":
		commentPrefixes = []string{"--"}
	}
	// Note: files like Makefile or shell scripts without extension might use #, handled roughly.
	if ext == "" || ext == "makefile" || ext == "dockerfile" {
		commentPrefixes = []string{"#"}
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		stats.TotalLines++

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			stats.BlankLines++
			continue
		}

		isComment := false
		for _, prefix := range commentPrefixes {
			if strings.HasPrefix(trimmed, prefix) {
				isComment = true
				break
			}
		}

		if isComment {
			stats.CommentLines++
		} else {
			stats.CodeLines++
		}
	}

	if err := scanner.Err(); err != nil {
		return stats, err
	}

	return stats, nil
}

func init() {
	rootCmd.AddCommand(slocCmd)
}
