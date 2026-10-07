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
			name, version := parseRequireLine(line, true)
			if name != "" && version != "" {
				pkgs = append(pkgs, Package{
					Name:      name,
					Version:   version,
					Ecosystem: "Go",
				})
			}
		} else if inRequire {
			// Block require: example.com/pkg v1.0.0
			name, version := parseRequireLine(line, false)
			if name != "" && version != "" {
				// Ignore // indirect comments?
				// Often vulnerabilities in indirect deps are relevant too.
				// Let's include them.
				pkgs = append(pkgs, Package{
					Name:      name,
					Version:   version,
					Ecosystem: "Go",
				})
			}
		}
	}
	return pkgs, scanner.Err()
}

// ⚡ Bolt: Helper to parse go.mod require lines without strings.Fields allocation
func parseRequireLine(line string, skipRequire bool) (string, string) {
	if skipRequire {
		if !strings.HasPrefix(line, "require ") {
			return "", ""
		}
		line = line[len("require "):]
	}

	// trim leading whitespace unconditionally
	for len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
		line = line[1:]
	}

	idx := strings.IndexAny(line, " \t")
	if idx != -1 {
		name := line[:idx]
		rem := line[idx+1:]
		for len(rem) > 0 && (rem[0] == ' ' || rem[0] == '\t') {
			rem = rem[1:]
		}
		idx2 := strings.IndexAny(rem, " \t")
		if idx2 != -1 {
			return name, rem[:idx2]
		}
		return name, rem
	}
	return "", ""
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
