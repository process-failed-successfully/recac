package runner

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"recac/internal/db"
	"recac/internal/telemetry"
	"time"

	"github.com/spf13/viper"
)

func initializeLogging(project string) *slog.Logger {
	// Create agents/logs directory in the current working directory (host)
	// This is where Promtail expects to find them based on docker-compose.monitoring.yml
	var agentsLogsDir string
	if logsDir := os.Getenv("RECAC_LOGS_DIR"); logsDir != "" {
		agentsLogsDir = logsDir
	} else {
		cwd, _ := os.Getwd()
		agentsLogsDir = filepath.Join(cwd, "agents", "logs")
	}

	if err := os.MkdirAll(agentsLogsDir, 0755); err != nil {
		fmt.Printf("Warning: Failed to create agents/logs directory: %v\n", err)
	} else {
		// Initialize session log file
		timestamp := time.Now().Format("20060102-150405")
		logFileName := fmt.Sprintf("%s_agent_%s_%s.log", project, project, timestamp)
		logFilePath := filepath.Join(agentsLogsDir, logFileName)

		// Re-initialize telemetry logger with the session log file
		// Note: We use the global 'verbose' setting
		// We still init global logger for backward compatibility and simpler calls where session isn't available
		telemetry.InitLogger(viper.GetBool("verbose"), logFilePath, false)
		fmt.Printf("Session logs will be written to: %s\n", logFilePath)
	}

	// Create session logger
	// We want to persist it in the session so it can be customized (e.g. with attributes)
	// For now, we reuse the configuration logic but ideally we'd pass this logger instance around.
	// Since we called InitLogger above, slog.Default() is set.
	// But let's create an explicit one too.
	logger := telemetry.NewLogger(viper.GetBool("verbose"), "", false)
	if project != "" {
		logger = logger.With("project", project)
	}
	return logger
}

func getDBConfig(workspace string) db.StoreConfig {
	dbType := os.Getenv("RECAC_DB_TYPE")
	dbURL := os.Getenv("RECAC_DB_URL")

	if dbType == "" {
		dbType = "sqlite"
		if dbURL == "" {
			dbURL = filepath.Join(workspace, ".recac.db")
		}
	} else if dbType == "sqlite" && dbURL == "" {
		dbURL = filepath.Join(workspace, ".recac.db")
	}

	return db.StoreConfig{
		Type:             dbType,
		ConnectionString: dbURL,
	}
}
