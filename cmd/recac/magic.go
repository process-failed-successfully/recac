package main

import (
	"encoding/json"
	"fmt"
	"recac/internal/analysis"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var (
	magicPath      string
	magicMinCount  int
	magicIgnore    string
	magicJSON      bool
	magicFail      bool
)

var magicCmd = &cobra.Command{
	Use:   "magic",
	Short: "Detect magic literals in Go code",
	Long: `Scans Go files for "magic" literals (unnamed numbers and strings) that should be constants.
It ignores common values (0, 1, "") and values defined in const blocks or struct tags.`,
	RunE: runMagic,
}

func init() {
	rootCmd.AddCommand(magicCmd)
	magicCmd.Flags().StringVarP(&magicPath, "path", "p", ".", "Path to analyze")
	magicCmd.Flags().IntVarP(&magicMinCount, "min", "m", 2, "Minimum occurrences to report")
	magicCmd.Flags().StringVarP(&magicIgnore, "ignore", "i", "", "Comma-separated list of values to ignore")
	magicCmd.Flags().BoolVar(&magicJSON, "json", false, "Output results as JSON")
	magicCmd.Flags().BoolVar(&magicFail, "fail", false, "Exit with error code if magic literals found")
}

func runMagic(cmd *cobra.Command, args []string) error {
	ignores := []string{}
	if magicIgnore != "" {
		for _, s := range strings.Split(magicIgnore, ",") {
			ignores = append(ignores, strings.TrimSpace(s))
		}
	}

	findings, err := analysis.ExtractMagicLiterals(magicPath, ignores)
	if err != nil {
		return err
	}

	// Filter by min count
	var filtered []analysis.MagicFinding
	for _, f := range findings {
		if f.Occurrences >= magicMinCount {
			filtered = append(filtered, f)
		}
	}

	// Sort by occurrences (descending)
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Occurrences > filtered[j].Occurrences
	})

	if magicJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(filtered)
	}

	if len(filtered) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "✅ No magic literals found! Clean code.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "VALUE\tTYPE\tCOUNT\tLOCATIONS")
	for _, f := range filtered {
		locs := f.Locations
		if len(locs) > 3 {
			locs = append(locs[:3], fmt.Sprintf("... (+%d)", len(locs)-3))
		}
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\n", f.Value, f.Type, f.Occurrences, strings.Join(locs, ", "))
	}
	w.Flush()

	if magicFail {
		return fmt.Errorf("found %d magic literals", len(filtered))
	}

	return nil
}
