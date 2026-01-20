package vuln

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
)

// GoModParser parses go.mod files.
type GoModParser struct{}

func (p *GoModParser) Parse(path string) ([]Package, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var pkgs []Package
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
			// Single line require: require example.com/pkg v1.0.0
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				pkgs = append(pkgs, Package{
					Name:      parts[1],
					Version:   parts[2],
					Ecosystem: "Go",
				})
			}
		} else if inRequire {
			// Block require: example.com/pkg v1.0.0
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				// Ignore // indirect comments?
				// Often vulnerabilities in indirect deps are relevant too.
				// Let's include them.
				pkgs = append(pkgs, Package{
					Name:      parts[0],
					Version:   parts[1],
					Ecosystem: "Go",
				})
			}
		}
	}
	return pkgs, scanner.Err()
}

// PackageJsonParser parses package.json files.
type PackageJsonParser struct{}

type packageJson struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

func (p *PackageJsonParser) Parse(path string) ([]Package, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var data packageJson
	if err := json.NewDecoder(f).Decode(&data); err != nil {
		return nil, err
	}

	var pkgs []Package
	for name, version := range data.Dependencies {
		pkgs = append(pkgs, Package{
			Name:      name,
			Version:   cleanNpmVersion(version),
			Ecosystem: "npm",
		})
	}
	for name, version := range data.DevDependencies {
		pkgs = append(pkgs, Package{
			Name:      name,
			Version:   cleanNpmVersion(version),
			Ecosystem: "npm",
		})
	}

	return pkgs, nil
}

func cleanNpmVersion(v string) string {
	// Remove ^, ~, >= etc. Very basic cleaning.
	// OSV API handles some ranges but precise version is better.
	// We'll strip common prefixes.
	v = strings.TrimPrefix(v, "^")
	v = strings.TrimPrefix(v, "~")
	v = strings.TrimPrefix(v, ">=")
	v = strings.TrimPrefix(v, ">")
	v = strings.TrimPrefix(v, "=")
	return v
}
