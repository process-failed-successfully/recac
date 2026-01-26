package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/vuln"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	sbomFormat string
	sbomOutput string
)

var sbomCmd = &cobra.Command{
	Use:   "sbom [path]",
	Short: "Generate a Software Bill of Materials (SBOM)",
	Long: `Generates a Software Bill of Materials (SBOM) for the current project or specified directory.
Scans 'go.mod' and 'package.json' to inventory dependencies.
Supports SPDX (Lite) and CycloneDX (Lite) JSON formats.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSBOM,
}

func init() {
	rootCmd.AddCommand(sbomCmd)
	sbomCmd.Flags().StringVar(&sbomFormat, "format", "spdx", "Output format: spdx, cyclonedx, or json")
	sbomCmd.Flags().StringVarP(&sbomOutput, "output", "o", "", "Output file path (default: stdout)")
}

func runSBOM(cmd *cobra.Command, args []string) error {
	var root string
	if len(args) > 0 {
		root = args[0]
	} else {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		root = wd
	}

	projectName := filepath.Base(root)

	// 1. Collect Packages
	packages := collectPackages(cmd, root)
	if len(packages) == 0 {
		return fmt.Errorf("no dependencies found (checked go.mod, package.json)")
	}

	// 2. Format Output
	var output []byte
	var err error
	switch strings.ToLower(sbomFormat) {
	case "spdx", "spdx-json":
		output, err = formatSPDX(projectName, packages)
	case "cyclonedx", "cyclonedx-json":
		output, err = formatCycloneDX(projectName, packages)
	case "json":
		output, err = json.MarshalIndent(packages, "", "  ")
	default:
		return fmt.Errorf("unknown format: %s", sbomFormat)
	}

	if err != nil {
		return fmt.Errorf("failed to format output: %w", err)
	}

	// 3. Write Output
	if sbomOutput != "" {
		if err := os.WriteFile(sbomOutput, output, 0644); err != nil {
			return fmt.Errorf("failed to write output file: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "SBOM written to %s\n", sbomOutput)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), string(output))
	}

	return nil
}

func collectPackages(cmd *cobra.Command, root string) []vuln.Package {
	var allPackages []vuln.Package

	// Check go.mod
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
		parser := &vuln.GoModParser{}
		pkgs, err := parser.Parse(filepath.Join(root, "go.mod"))
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to parse go.mod: %v\n", err)
		} else {
			allPackages = append(allPackages, pkgs...)
		}
	}

	// Check package.json
	if _, err := os.Stat(filepath.Join(root, "package.json")); err == nil {
		parser := &vuln.PackageJsonParser{}
		pkgs, err := parser.Parse(filepath.Join(root, "package.json"))
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to parse package.json: %v\n", err)
		} else {
			allPackages = append(allPackages, pkgs...)
		}
	}

	return allPackages
}

// SPDX Structures
type SPDXDocument struct {
	SPDXID        string        `json:"SPDXID"`
	SPDXVersion   string        `json:"spdxVersion"`
	Name          string        `json:"name"`
	CreationInfo  SPDXCreation  `json:"creationInfo"`
	Packages      []SPDXPackage `json:"packages"`
	Relationships []SPDXRel     `json:"relationships"`
}

type SPDXCreation struct {
	Created            string   `json:"created"`
	Creators           []string `json:"creators"`
	LicenseListVersion string   `json:"licenseListVersion"`
}

type SPDXPackage struct {
	Name             string `json:"name"`
	SPDXID           string `json:"SPDXID"`
	VersionInfo      string `json:"versionInfo"`
	DownloadLocation string `json:"downloadLocation"`
	LicenseConcluded string `json:"licenseConcluded"`
}

type SPDXRel struct {
	ElementID      string `json:"spdxElementId"`
	RelatedID      string `json:"relatedSpdxElement"`
	RelationshipType string `json:"relationshipType"`
}

func formatSPDX(projectName string, packages []vuln.Package) ([]byte, error) {
	doc := SPDXDocument{
		SPDXID:      "SPDXRef-DOCUMENT",
		SPDXVersion: "SPDX-2.3",
		Name:        projectName,
		CreationInfo: SPDXCreation{
			Created:  time.Now().UTC().Format(time.RFC3339),
			Creators: []string{"Tool: recac-sbom"},
		},
		Packages:      []SPDXPackage{},
		Relationships: []SPDXRel{},
	}

	// Add Root Package (Project itself)
	rootPkgID := "SPDXRef-RootPackage"
	doc.Packages = append(doc.Packages, SPDXPackage{
		Name:             projectName,
		SPDXID:           rootPkgID,
		VersionInfo:      "0.0.0-dev",
		DownloadLocation: "NOASSERTION",
		LicenseConcluded: "NOASSERTION",
	})
	doc.Relationships = append(doc.Relationships, SPDXRel{
		ElementID:        "SPDXRef-DOCUMENT",
		RelatedID:        rootPkgID,
		RelationshipType: "DESCRIBES",
	})

	for i, p := range packages {
		pkgID := fmt.Sprintf("SPDXRef-Package-%d", i)
		doc.Packages = append(doc.Packages, SPDXPackage{
			Name:             p.Name,
			SPDXID:           pkgID,
			VersionInfo:      p.Version,
			DownloadLocation: "NOASSERTION", // In real SBOM, we'd guess git url
			LicenseConcluded: "NOASSERTION",
		})

		doc.Relationships = append(doc.Relationships, SPDXRel{
			ElementID:        rootPkgID,
			RelatedID:        pkgID,
			RelationshipType: "DEPENDS_ON",
		})
	}

	return json.MarshalIndent(doc, "", "  ")
}

// CycloneDX Structures
type CycloneDXDocument struct {
	BomFormat   string               `json:"bomFormat"`
	SpecVersion string               `json:"specVersion"`
	Metadata    CycloneDXMetadata    `json:"metadata"`
	Components  []CycloneDXComponent `json:"components"`
}

type CycloneDXMetadata struct {
	Timestamp string            `json:"timestamp"`
	Tool      CycloneDXTool     `json:"tool"`
	Component CycloneDXComponent `json:"component"`
}

type CycloneDXTool struct {
	Vendor  string `json:"vendor"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

type CycloneDXComponent struct {
	Type    string `json:"type"` // library, application
	Name    string `json:"name"`
	Version string `json:"version"`
	Purl    string `json:"purl,omitempty"` // Package URL
}

func formatCycloneDX(projectName string, packages []vuln.Package) ([]byte, error) {
	doc := CycloneDXDocument{
		BomFormat:   "CycloneDX",
		SpecVersion: "1.5",
		Metadata: CycloneDXMetadata{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Tool: CycloneDXTool{
				Vendor:  "Recac",
				Name:    "recac",
				Version: "latest",
			},
			Component: CycloneDXComponent{
				Type:    "application",
				Name:    projectName,
				Version: "0.0.0-dev",
			},
		},
		Components: []CycloneDXComponent{},
	}

	for _, p := range packages {
		// Construct minimal PURL
		// pkg:type/namespace/name@version
		purl := ""
		if p.Ecosystem == "Go" {
			purl = fmt.Sprintf("pkg:golang/%s@%s", p.Name, p.Version)
		} else if p.Ecosystem == "npm" {
			purl = fmt.Sprintf("pkg:npm/%s@%s", p.Name, p.Version)
		}

		doc.Components = append(doc.Components, CycloneDXComponent{
			Type:    "library",
			Name:    p.Name,
			Version: p.Version,
			Purl:    purl,
		})
	}

	return json.MarshalIndent(doc, "", "  ")
}
