package analysis

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

// RadarItem represents a single technology on the radar.
type RadarItem struct {
	Name        string `json:"name"`
	Quadrant    string `json:"quadrant"` // Languages, Platforms, Techniques, Tools
	Ring        string `json:"ring"`     // Adopt, Trial, Assess, Hold
	Description string `json:"description"`
}

// RadarReport holds all items for the radar.
type RadarReport struct {
	Items []RadarItem `json:"items"`
}

// ScanForDependencyFiles searches the root directory for known dependency files.
func ScanForDependencyFiles(root string) ([]string, error) {
	var files []string
	knownFiles := map[string]bool{
		"go.mod":             true,
		"package.json":       true,
		"requirements.txt":   true,
		"pom.xml":            true,
		"Dockerfile":         true,
		"docker-compose.yml": true,
		"Gemfile":            true,
		"Cargo.toml":         true,
		"build.gradle":       true,
		"Makefile":           true,
		".travis.yml":        true,
		".github/workflows":  true, // Maybe scan workflow files?
	}

	// Just check the root level for now to avoid massive traversal?
	// Or define a recursive scan with limits?
	// Let's stick to root + common config dirs for simplicity and speed.
	// Users usually put dependency files at root.
	// We can use filepath.Walk but limit depth?
	// Let's implement a shallow scan + specific nested paths.

	// 1. Root scan
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			name := entry.Name()
			if knownFiles[name] || strings.HasSuffix(name, ".tf") || strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml") {
				files = append(files, filepath.Join(root, name))
			}
		}
	}

	// 2. Specific nested paths (e.g. workflows)
	workflowsPath := filepath.Join(root, ".github", "workflows")
	if info, err := os.Stat(workflowsPath); err == nil && info.IsDir() {
		wfEntries, _ := os.ReadDir(workflowsPath)
		for _, entry := range wfEntries {
			if !entry.IsDir() && (strings.HasSuffix(entry.Name(), ".yml") || strings.HasSuffix(entry.Name(), ".yaml")) {
				files = append(files, filepath.Join(workflowsPath, entry.Name()))
			}
		}
	}

	return files, nil
}

// GenerateRadarHTML generates an interactive HTML report.
func GenerateRadarHTML(items []RadarItem) (string, error) {
	tmpl, err := template.New("radar").Parse(radarHTMLTemplate)
	if err != nil {
		return "", err
	}

	// Convert items to JSON for JS consumption
	jsonData, err := json.Marshal(items)
	if err != nil {
		return "", err
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, map[string]interface{}{
		"ItemsJSON": string(jsonData),
	}); err != nil {
		return "", err
	}

	return buf.String(), nil
}

const radarHTMLTemplate = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Technology Radar</title>
    <style>
        body { font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; background-color: #f4f4f9; color: #333; margin: 0; padding: 20px; }
        .container { max-width: 1200px; margin: 0 auto; }
        h1 { text-align: center; color: #2c3e50; }
        .radar-container { display: flex; flex-wrap: wrap; justify-content: space-around; margin-top: 40px; }
        .quadrant { background: #fff; border-radius: 8px; box-shadow: 0 4px 6px rgba(0,0,0,0.1); width: 45%; margin-bottom: 30px; padding: 20px; box-sizing: border-box; }
        .quadrant h2 { border-bottom: 2px solid #eee; padding-bottom: 10px; margin-top: 0; color: #34495e; }
        .ring-group { margin-bottom: 15px; }
        .ring-title { font-weight: bold; font-size: 0.9em; text-transform: uppercase; color: #7f8c8d; margin-bottom: 5px; }
        .tech-item { display: inline-block; background: #ecf0f1; border-radius: 4px; padding: 4px 8px; margin: 2px; font-size: 0.9em; cursor: pointer; transition: background 0.2s; }
        .tech-item:hover { background: #bdc3c7; }
        .tech-item.adopt { border-left: 4px solid #2ecc71; }
        .tech-item.trial { border-left: 4px solid #f1c40f; }
        .tech-item.assess { border-left: 4px solid #3498db; }
        .tech-item.hold { border-left: 4px solid #e74c3c; }

        /* Modal */
        .modal { display: none; position: fixed; z-index: 1; left: 0; top: 0; width: 100%; height: 100%; overflow: auto; background-color: rgba(0,0,0,0.4); }
        .modal-content { background-color: #fefefe; margin: 15% auto; padding: 20px; border: 1px solid #888; width: 80%; max-width: 600px; border-radius: 8px; position: relative; }
        .close { color: #aaa; float: right; font-size: 28px; font-weight: bold; cursor: pointer; }
        .close:hover, .close:focus { color: black; text-decoration: none; cursor: pointer; }
        #modalTitle { margin-top: 0; }
        #modalBadge { display: inline-block; padding: 4px 8px; border-radius: 4px; color: white; font-weight: bold; font-size: 0.8em; margin-bottom: 10px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Technology Radar</h1>
        <p style="text-align: center; color: #7f8c8d;">An overview of the technologies, tools, and platforms used in this project.</p>

        <div id="radar" class="radar-container">
            <!-- Quadrants will be injected here -->
        </div>
    </div>

    <div id="itemModal" class="modal">
        <div class="modal-content">
            <span class="close">&times;</span>
            <h2 id="modalTitle">Tech Name</h2>
            <span id="modalBadge">Ring</span>
            <p id="modalDesc">Description goes here...</p>
        </div>
    </div>

    <script>
        const data = {{.ItemsJSON}};

        const quadrants = {
            'Languages & Frameworks': [],
            'Tools': [],
            'Platforms': [],
            'Techniques': []
        };

        // Normalize quadrant names from AI output
        function normalizeQuadrant(q) {
            q = q.toLowerCase();
            if (q.includes('lang') || q.includes('frame')) return 'Languages & Frameworks';
            if (q.includes('tool')) return 'Tools';
            if (q.includes('plat') || q.includes('infra')) return 'Platforms';
            if (q.includes('tech') || q.includes('method')) return 'Techniques';
            return 'Tools'; // Default
        }

        // Normalize ring names
        function normalizeRing(r) {
            r = r.toLowerCase();
            if (r.includes('adopt')) return 'adopt';
            if (r.includes('trial')) return 'trial';
            if (r.includes('assess')) return 'assess';
            if (r.includes('hold')) return 'hold';
            return 'assess';
        }

        data.forEach(item => {
            const q = normalizeQuadrant(item.quadrant);
            item.ring = normalizeRing(item.ring);
            if (!quadrants[q]) quadrants[q] = []; // Just in case
            quadrants[q].push(item);
        });

        const container = document.getElementById('radar');
        const ringColors = {
            'adopt': '#2ecc71',
            'trial': '#f1c40f',
            'assess': '#3498db',
            'hold': '#e74c3c'
        };

        for (const [qName, items] of Object.entries(quadrants)) {
            const qDiv = document.createElement('div');
            qDiv.className = 'quadrant';
            qDiv.innerHTML = '<h2>' + qName + '</h2>';

            const rings = { 'adopt': [], 'trial': [], 'assess': [], 'hold': [] };
            items.forEach(item => rings[item.ring].push(item));

            ['adopt', 'trial', 'assess', 'hold'].forEach(ring => {
                if (rings[ring].length > 0) {
                    const rDiv = document.createElement('div');
                    rDiv.className = 'ring-group';
                    rDiv.innerHTML = '<div class="ring-title" style="color:' + ringColors[ring] + '">' + ring + '</div>';

                    rings[ring].forEach(item => {
                        const iSpan = document.createElement('span');
                        iSpan.className = 'tech-item ' + ring;
                        iSpan.textContent = item.name;
                        iSpan.onclick = () => showModal(item);
                        rDiv.appendChild(iSpan);
                    });
                    qDiv.appendChild(rDiv);
                }
            });
            container.appendChild(qDiv);
        }

        // Modal Logic
        const modal = document.getElementById("itemModal");
        const span = document.getElementsByClassName("close")[0];

        function showModal(item) {
            document.getElementById("modalTitle").textContent = item.name;
            const badge = document.getElementById("modalBadge");
            badge.textContent = item.ring.toUpperCase();
            badge.style.backgroundColor = ringColors[item.ring];
            document.getElementById("modalDesc").textContent = item.description || "No description provided.";
            modal.style.display = "block";
        }

        span.onclick = function() { modal.style.display = "none"; }
        window.onclick = function(event) { if (event.target == modal) modal.style.display = "none"; }
    </script>
</body>
</html>
`
