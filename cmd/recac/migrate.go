package main

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"recac/internal/agent"
	"recac/internal/migration"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

var (
	migrateDB    string
	migrateDir   string
	migrateAI    string
	migrateSteps int
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Manage database migrations",
	Long:  `Create, apply, and rollback database migrations with AI support.`,
}

var migrateCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new migration",
	Long:  `Creates a new timestamped migration file pair (up/down).
Optionally uses AI to generate the SQL content.`,
	Example: `  recac migrate create add_users_table --ai "Create users table with id, name, email"`,
	Args:    cobra.ExactArgs(1),
	RunE:    runMigrateCreate,
}

var migrateUpCmd = &cobra.Command{
	Use:   "up",
	Short: "Apply pending migrations",
	RunE:  runMigrateUp,
}

var migrateDownCmd = &cobra.Command{
	Use:   "down",
	Short: "Rollback applied migrations",
	RunE:  runMigrateDown,
}

var migrateStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show migration status",
	RunE:  runMigrateStatus,
}

func init() {
	rootCmd.AddCommand(migrateCmd)
	migrateCmd.PersistentFlags().StringVar(&migrateDB, "db", "", "Database connection string or file path")
	migrateCmd.PersistentFlags().StringVar(&migrateDir, "dir", "migrations", "Migrations directory")

	migrateCmd.AddCommand(migrateCreateCmd)
	migrateCreateCmd.Flags().StringVar(&migrateAI, "ai", "", "Prompt for AI to generate SQL")

	migrateCmd.AddCommand(migrateUpCmd)
	migrateUpCmd.Flags().IntVarP(&migrateSteps, "steps", "n", 0, "Number of migrations to apply (0 = all)")

	migrateCmd.AddCommand(migrateDownCmd)
	migrateDownCmd.Flags().IntVarP(&migrateSteps, "steps", "n", 1, "Number of migrations to rollback")

	migrateCmd.AddCommand(migrateStatusCmd)
}

func getMigrator() (*migration.Migrator, error) {
	connStr := migrateDB
	if connStr == "" {
		// Try defaults
		if _, err := os.Stat("recac.db"); err == nil {
			connStr = "recac.db"
		} else if _, err := os.Stat(".recac.db"); err == nil {
			connStr = ".recac.db"
		} else {
			return nil, fmt.Errorf("connection string or file path required (use --db)")
		}
	}

	var dbType string
	if strings.HasPrefix(connStr, "postgres://") || strings.HasPrefix(connStr, "postgresql://") {
		dbType = "postgres"
	} else {
		dbType = "sqlite"
	}

	db, err := sql.Open(dbType, connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	return migration.NewMigrator(db, migrateDir, dbType), nil
}

func runMigrateCreate(cmd *cobra.Command, args []string) error {
	name := args[0]

	var ag agent.Agent
	if migrateAI != "" {
		cwd, _ := os.Getwd()
		ctx := cmd.Context()
		var err error
		ag, err = agentClientFactory(ctx, viper.GetString("provider"), viper.GetString("model"), cwd, "recac-migrate")
		if err != nil {
			return fmt.Errorf("failed to create agent: %w", err)
		}
	}

	version, err := migration.Generate(migrateDir, name, migrateAI, ag)
	if err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Created migration %s_%s\n", version, strings.ReplaceAll(name, " ", "_"))
	return nil
}

func runMigrateUp(cmd *cobra.Command, args []string) error {
	m, err := getMigrator()
	if err != nil {
		return err
	}
	defer m.DB.Close()

	if err := m.Up(migrateSteps); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Migrations up-to-date.")
	return nil
}

func runMigrateDown(cmd *cobra.Command, args []string) error {
	m, err := getMigrator()
	if err != nil {
		return err
	}
	defer m.DB.Close()

	if err := m.Down(migrateSteps); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Rollback complete.")
	return nil
}

func runMigrateStatus(cmd *cobra.Command, args []string) error {
	m, err := getMigrator()
	if err != nil {
		return err
	}
	defer m.DB.Close()

	status, err := m.GetStatus()
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "VERSION\tNAME\tSTATUS\tAPPLIED AT")
	fmt.Fprintln(w, "-------\t----\t------\t----------")

	// Merge lists for display (naive sort by version)
	// Both lists are already sorted by version in Migrator.GetStatus logic (implied for Applied, explicitly for Pending)
	// We just need to merge print.

	// Helper to print
	printRow := func(mig migration.Migration, applied bool) {
		statusStr := "Pending"
		appliedAtStr := ""
		if applied {
			statusStr = "Applied"
			appliedAtStr = mig.AppliedAt.Format(time.RFC3339)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", mig.Version, mig.Name, statusStr, appliedAtStr)
	}

	// We can just print applied then pending, as pending are always newer?
	// Not necessarily if we insert an older migration file later (but that's bad practice).
	// Assuming pending versions are > applied versions.

	for _, mig := range status.Applied {
		printRow(mig, true)
	}
	for _, mig := range status.Pending {
		printRow(mig, false)
	}

	w.Flush()
	return nil
}
