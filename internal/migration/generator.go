package migration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"recac/internal/agent"
)

// Generate creates new migration files.
func Generate(dir, name string, aiPrompt string, ag agent.Agent) (string, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	// Sanitize name
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ToLower(name)

	version := time.Now().Format("20060102150405")
	baseName := fmt.Sprintf("%s_%s", version, name)
	upFile := filepath.Join(dir, baseName+".up.sql")
	downFile := filepath.Join(dir, baseName+".down.sql")

	upContent := "-- Up migration\n"
	downContent := "-- Down migration\n"

	if aiPrompt != "" && ag != nil {
		ctx := context.Background()
		prompt := fmt.Sprintf(`Generate SQL migration for: "%s".
Return the raw SQL for the UP migration, followed by a separator "--- DOWN ---", followed by the raw SQL for the DOWN migration.
Do not include any markdown formatting or code blocks.
Example:
CREATE TABLE users (id INT);
--- DOWN ---
DROP TABLE users;`, aiPrompt)

		resp, err := ag.Send(ctx, prompt)
		if err == nil {
			// Clean up response just in case
			resp = strings.ReplaceAll(resp, "```sql", "")
			resp = strings.ReplaceAll(resp, "```", "")

			parts := strings.Split(resp, "--- DOWN ---")
			if len(parts) >= 1 {
				upContent = strings.TrimSpace(parts[0])
			}
			if len(parts) >= 2 {
				downContent = strings.TrimSpace(parts[1])
			}
		} else {
			upContent += fmt.Sprintf("\n-- AI Generation Failed: %v\n", err)
		}
	}

	if err := os.WriteFile(upFile, []byte(upContent), 0644); err != nil {
		return "", err
	}
	if err := os.WriteFile(downFile, []byte(downContent), 0644); err != nil {
		return "", err
	}

	return version, nil
}
