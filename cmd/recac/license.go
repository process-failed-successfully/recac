package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	licenseAllow    []string
	licenseDeny     []string
	licensePath     string
	licenseFail     bool
	licenseJSON     bool
	licenseNoAI     bool
)

var licenseCmd = &cobra.Command{
	Use:   "license",
	Short: "Manage and check dependency licenses",
	Long:  `Scan dependencies for license files and check compliance against a policy.`,
}

var licenseCheckCmd = &cobra.Command{
	Use:   "check [path]",
	Short: "Check licenses of dependencies",
	Long: `Scans 'vendor' (Go) or 'node_modules' (Node.js) directories for LICENSE files.
Identifies licenses using Regex matching and falls back to AI for unknown text.
Checks against allowed/denied lists.`,
	RunE: runLicenseCheck,
}

func init() {
	rootCmd.AddCommand(licenseCmd)
	licenseCmd.AddCommand(licenseCheckCmd)

	licenseCheckCmd.Flags().StringSliceVar(&licenseAllow, "allow", []string{"MIT", "Apache-2.0", "BSD", "ISC", "0BSD"}, "List of allowed licenses")
	licenseCheckCmd.Flags().StringSliceVar(&licenseDeny, "deny", []string{"GPL", "AGPL", "Proprietary"}, "List of explicitly denied licenses")
	licenseCheckCmd.Flags().BoolVar(&licenseFail, "fail", false, "Exit with error if non-compliant licenses found")
	licenseCheckCmd.Flags().BoolVar(&licenseJSON, "json", false, "Output as JSON")
	licenseCheckCmd.Flags().BoolVar(&licenseNoAI, "no-ai", false, "Disable AI fallback for license identification")
}

type LicenseResult struct {
	Package   string `json:"package"`
	License   string `json:"license"`
	File      string `json:"file"`
	Status    string `json:"status"` // "Allowed", "Denied", "Review"
	Method    string `json:"method"` // "Regex", "AI", "Unknown"
	Confidence string `json:"confidence,omitempty"`
}

func runLicenseCheck(cmd *cobra.Command, args []string) error {
	root := "."
	if len(args) > 0 {
		root = args[0]
	}

	// 1. Find License Files
	files, err := findLicenseFiles(root)
	if err != nil {
		return err
	}

	if len(files) == 0 {
		if !licenseJSON {
			fmt.Fprintln(cmd.OutOrStdout(), "No dependency license files found. (Checked vendor/ and node_modules/)")
			fmt.Fprintln(cmd.OutOrStdout(), "Tip: Run 'go mod vendor' or 'npm install' to populate dependencies locally.")
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "[]")
		}
		return nil
	}

	// 2. Identify and Check
	var results []LicenseResult

	// Pre-compile regexes
	regexes := map[string]*regexp.Regexp{
		"MIT":        regexp.MustCompile(`(?i)\bMIT\b`),
		"Apache-2.0": regexp.MustCompile(`(?i)Apache License`),
		"BSD-3-Clause": regexp.MustCompile(`(?i)BSD.*3.*Clause`),
		"BSD-2-Clause": regexp.MustCompile(`(?i)BSD.*2.*Clause`),
		"ISC":        regexp.MustCompile(`(?i)\bISC\b`),
		"GPL-3.0":    regexp.MustCompile(`(?i)GNU General Public License.*Version 3`),
		"GPL-2.0":    regexp.MustCompile(`(?i)GNU General Public License.*Version 2`),
		"AGPL-3.0":   regexp.MustCompile(`(?i)Affero General Public License`),
		"MPL-2.0":    regexp.MustCompile(`(?i)Mozilla Public License`),
	}

	ctx := context.Background()

	// Initialize agent only if needed and not disabled
	// We'll do it lazily or here. Let's do it here to catch config errors early if AI is expected.
	// But actually, we only need it for fallback.

	for _, file := range files {
		contentBytes, err := os.ReadFile(file)
		if err != nil {
			continue // Skip unreadable
		}
		content := string(contentBytes)

		pkgName := getPackageNameFromFile(root, file)

		licType := "Unknown"
		method := "Unknown"

		// Regex Match
		for name, re := range regexes {
			if re.MatchString(content) {
				licType = name
				method = "Regex"
				break
			}
		}

		// AI Fallback
		if licType == "Unknown" && !licenseNoAI {
			aiLic, err := identifyLicenseWithAI(ctx, content)
			if err == nil {
				licType = aiLic
				method = "AI"
			}
		}

		// Determine Status
		status := "Review"
		if isDenied(licType) {
			status = "Denied"
		} else if isAllowed(licType) {
			status = "Allowed"
		}

		results = append(results, LicenseResult{
			Package: pkgName,
			License: licType,
			File:    file,
			Status:  status,
			Method:  method,
		})
	}

	// 3. Output
	if licenseJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		if err := enc.Encode(results); err != nil {
			return err
		}
	} else {
		printLicenseTable(cmd, results)
	}

	if licenseFail {
		for _, r := range results {
			if r.Status == "Denied" {
				return fmt.Errorf("license compliance failed: denied licenses found")
			}
		}
	}

	return nil
}

func findLicenseFiles(root string) ([]string, error) {
	var files []string

	// We specifically look into vendor/ and node_modules/
	// Walking the entire tree is too slow and catches the project's own license multiple times maybe.

	searchPaths := []string{
		filepath.Join(root, "vendor"),
		filepath.Join(root, "node_modules"),
	}

	for _, searchPath := range searchPaths {
		if _, err := os.Stat(searchPath); os.IsNotExist(err) {
			continue
		}

		filepath.WalkDir(searchPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}

			if d.IsDir() {
				// Don't skip hidden directories like .bin in node_modules?
				// Actually node_modules/.bin/ doesn't have licenses usually.
				if strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
					return filepath.SkipDir
				}
				return nil
			}

			name := strings.ToUpper(d.Name())
			if strings.Contains(name, "LICENSE") || strings.Contains(name, "COPYING") || strings.Contains(name, "LICENCE") {
				files = append(files, path)
			}

			return nil
		})
	}

	return files, nil
}

func getPackageNameFromFile(root, path string) string {
	rel, _ := filepath.Rel(root, path)
	// vendor/github.com/user/repo/LICENSE -> github.com/user/repo
	// node_modules/package/LICENSE -> package

	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) > 1 {
		if parts[0] == "vendor" {
			// Find the directory containing the license
			return filepath.Dir(strings.Join(parts[1:], "/"))
		} else if parts[0] == "node_modules" {
			// Usually node_modules/pkg/LICENSE
			if len(parts) >= 3 && strings.HasPrefix(parts[1], "@") {
				// Scoped package: node_modules/@scope/pkg/LICENSE
				return parts[1] + "/" + parts[2]
			}
			return parts[1]
		}
	}
	return filepath.Dir(rel)
}

func isAllowed(license string) bool {
	for _, a := range licenseAllow {
		if strings.EqualFold(a, license) {
			return true
		}
	}
	return false
}

func isDenied(license string) bool {
	for _, d := range licenseDeny {
		if strings.EqualFold(d, license) {
			return true
		}
	}
	return false
}

func identifyLicenseWithAI(ctx context.Context, content string) (string, error) {
	// Truncate content if too long
	if len(content) > 2000 {
		content = content[:2000] + "...(truncated)"
	}

	provider := viper.GetString("provider")
	model := viper.GetString("model")
	cwd, _ := os.Getwd()

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-license")
	if err != nil {
		return "", err
	}

	prompt := fmt.Sprintf(`Identify the software license from the following text.
Return ONLY the SPDX identifier (e.g., "MIT", "Apache-2.0", "GPL-3.0", "BSD-3-Clause").
If it is not a standard license, return "Proprietary" or "Unknown".
Do not include any explanation.

License Text:
'''
%s
'''`, content)

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(strings.Trim(resp, "\"`'")), nil
}

func printLicenseTable(cmd *cobra.Command, results []LicenseResult) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "STATUS\tPACKAGE\tLICENSE\tMETHOD")

	// Sort by Status (Denied first), then Package
	sort.Slice(results, func(i, j int) bool {
		if results[i].Status != results[j].Status {
			// Order: Denied, Review, Allowed
			order := map[string]int{"Denied": 0, "Review": 1, "Allowed": 2}
			return order[results[i].Status] < order[results[j].Status]
		}
		return results[i].Package < results[j].Package
	})

	for _, r := range results {
		icon := "✅"
		if r.Status == "Denied" {
			icon = "❌"
		} else if r.Status == "Review" {
			icon = "⚠️ "
		}

		fmt.Fprintf(w, "%s %s\t%s\t%s\t%s\n", icon, r.Status, r.Package, r.License, r.Method)
	}
	w.Flush()
}
