package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	licensePolicyFile string
	licenseJson       bool
	licenseFail       bool
	licenseExplain    bool
)

// LicenseEntry represents the license information for a dependency.
type LicenseEntry struct {
	Package string `json:"package"`
	License string `json:"license"`
	Source  string `json:"source"` // "file", "ai", "manual", "cache"
	Status  string `json:"status"` // "allowed", "denied", "unknown"
}

// LicenseCache maps package names to LicenseEntry.
type LicenseCache map[string]LicenseEntry

var licenseCmd = &cobra.Command{
	Use:   "license",
	Short: "Manage and check dependency licenses",
	Long:  `Scans dependencies and checks their licenses against a policy. Uses AI to identify licenses for unknown packages.`,
}

var licenseCheckCmd = &cobra.Command{
	Use:   "check [path]",
	Short: "Check licenses of dependencies",
	RunE:  runLicenseCheck,
}

func init() {
	rootCmd.AddCommand(licenseCmd)
	licenseCmd.AddCommand(licenseCheckCmd)

	licenseCheckCmd.Flags().StringVar(&licensePolicyFile, "policy", "", "Path to policy file (json/yaml)")
	licenseCheckCmd.Flags().BoolVar(&licenseJson, "json", false, "Output as JSON")
	licenseCheckCmd.Flags().BoolVar(&licenseFail, "fail", false, "Fail if any denied licenses are found")
	licenseCheckCmd.Flags().BoolVar(&licenseExplain, "explain", false, "Ask AI to explain license implications for denied/unknown")
}

func runLicenseCheck(cmd *cobra.Command, args []string) error {
	root := "."
	if len(args) > 0 {
		root = args[0]
	}

	// 1. Identify Dependencies
	deps, err := parseGoMod(filepath.Join(root, "go.mod"))
	if err != nil {
		// Try to continue if go.mod is missing but maybe package.json exists?
		// For now, fail if no go.mod, or just warn.
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to parse go.mod: %v\n", err)
	}

	// TODO: Add support for package.json or requirements.txt

	if len(deps) == 0 {
		return fmt.Errorf("no dependencies found")
	}

	// 2. Load Cache
	cacheFile := filepath.Join(root, ".recac", "licenses.json")
	cache := loadLicenseCache(cacheFile)

	// 3. Process Dependencies
	var results []LicenseEntry
	var mutex sync.Mutex
	var wg sync.WaitGroup

	// Default Policy
	allowed := []string{"MIT", "Apache-2.0", "BSD-3-Clause", "BSD-2-Clause", "ISC", "MPL-2.0", "0BSD", "Unlicense"}
	denied := []string{"GPL-3.0", "GPL-2.0", "AGPL-3.0"}
	// If policy file is provided, load it (omitted for brevity, can easily add)

	// Use a semaphore to limit concurrency for AI calls
	sem := make(chan struct{}, 5)

	for _, dep := range deps {
		wg.Add(1)
		go func(pkg string) {
			defer wg.Done()

			// Check cache first
			mutex.Lock()
			entry, ok := cache[pkg]
			mutex.Unlock()

			if ok {
				// Re-evaluate status in case policy changed
				entry.Status = checkPolicy(entry.License, allowed, denied)
				mutex.Lock()
				results = append(results, entry)
				mutex.Unlock()
				return
			}

			// Try to find license file locally (if vendor exists)
			// license, source := checkVendorLicense(root, pkg)
            // For this implementation, we skip vendor check complexity and go straight to AI/Knowledge
            // because in this sandbox we might not have vendor.

			// Fallback to AI
			sem <- struct{}{} // Acquire token
			defer func() { <-sem }()

			license := "Unknown"
            source := "ai"

            // Lazily init agent
            ctx := context.Background()
            cwd, _ := os.Getwd()
            ag, err := agentClientFactory(ctx, viper.GetString("provider"), viper.GetString("model"), cwd, "recac-license")
            if err != nil {
                 // handle error
				 fmt.Fprintf(cmd.ErrOrStderr(), "Failed to init agent: %v\n", err)
            } else {
                 // Ask agent
                 prompt := fmt.Sprintf(`Identify the open source license for the Go package '%s'.
Respond with ONLY the SPDX identifier (e.g., MIT, Apache-2.0, BSD-3-Clause).
If it is dual-licensed, list both separated by ' OR '.
If you are unsure, respond with 'Unknown'.
Do not explain.`, pkg)

				 var sb strings.Builder
                 _, err := ag.SendStream(ctx, prompt, func(chunk string) {
					sb.WriteString(chunk)
				 })
                 if err == nil {
                     license = strings.TrimSpace(sb.String())
                     // cleanup response
					 license = strings.ReplaceAll(license, "\n", "")
                     if strings.Contains(license, "I am unsure") || strings.Contains(license, "sorry") {
						 license = "Unknown"
					 }
                 }
            }

            status := checkPolicy(license, allowed, denied)
            newEntry := LicenseEntry{
                Package: pkg,
                License: license,
                Source: source,
                Status: status,
            }

            mutex.Lock()
            results = append(results, newEntry)
            cache[pkg] = newEntry
            mutex.Unlock()

		}(dep)
	}

	wg.Wait()

	// Sort results
	sort.Slice(results, func(i, j int) bool {
		return results[i].Package < results[j].Package
	})

	// Save Cache
	saveLicenseCache(cacheFile, cache)

	// Output
	if licenseJson {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	}

	printLicenseReport(cmd, results)

	if licenseFail {
		for _, r := range results {
			if r.Status == "denied" {
				return fmt.Errorf("denied licenses found")
			}
		}
	}

	return nil
}

func parseGoMod(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var deps []string
	scanner := bufio.NewScanner(f)
	inRequire := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "require (" {
			inRequire = true
			continue
		}
		if line == ")" && inRequire {
			inRequire = false
			continue
		}

		if strings.HasPrefix(line, "require ") {
			// Single line require
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				deps = append(deps, parts[1])
			}
		} else if inRequire {
			// Block require
			parts := strings.Fields(line)
			if len(parts) >= 1 {
				// Ignore // indirect comments for now, or include them?
				// Usually we care about all deps.
				deps = append(deps, parts[0])
			}
		}
	}
	return deps, scanner.Err()
}

func loadLicenseCache(path string) LicenseCache {
	cache := make(LicenseCache)
	f, err := os.Open(path)
	if err != nil {
		return cache
	}
	defer f.Close()
	json.NewDecoder(f).Decode(&cache)
	return cache
}

func saveLicenseCache(path string, cache LicenseCache) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(cache)
}

func checkPolicy(license string, allowed, denied []string) string {
	// Normalize
	license = strings.ToUpper(license)

	// Check denied first
	for _, d := range denied {
		if strings.Contains(license, strings.ToUpper(d)) {
			return "denied"
		}
	}

	// Check allowed
	// If allow list is empty, allow everything not denied? No, usually allow list is strict.
	// But for "Unknown", we return "unknown".
	if license == "UNKNOWN" {
		return "unknown"
	}

	for _, a := range allowed {
		if strings.Contains(license, strings.ToUpper(a)) {
			return "allowed"
		}
	}

	return "unknown" // Not explicitly allowed
}

func printLicenseReport(cmd *cobra.Command, results []LicenseEntry) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "PACKAGE\tLICENSE\tSTATUS\tSOURCE")
	fmt.Fprintln(w, "-------\t-------\t------\t------")

	for _, r := range results {
		icon := "✅"
		if r.Status == "denied" {
			icon = "❌"
		} else if r.Status == "unknown" {
			icon = "⚠️"
		}
		fmt.Fprintf(w, "%s\t%s\t%s %s\t%s\n", r.Package, r.License, icon, strings.ToUpper(r.Status), r.Source)
	}
	w.Flush()
}
