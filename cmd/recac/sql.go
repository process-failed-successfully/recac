package main

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

var (
	sqlDbPath  string
	sqlExecute bool
	sqlOutput  string
)

var sqlCmd = &cobra.Command{
	Use:   "sql [question]",
	Short: "Generate and execute SQL from natural language",
	Long: `Generate SQL queries from natural language questions using your database schema.
Can optionally execute the generated query and display the results.

Examples:
  recac sql "Show me the top 5 users by order count" --db ./my.db
  recac sql "Delete all logs older than 30 days" --execute
`,
	Args: cobra.MinimumNArgs(1),
	RunE: runSQL,
}

func init() {
	rootCmd.AddCommand(sqlCmd)
	sqlCmd.Flags().StringVarP(&sqlDbPath, "db", "d", "", "Database connection string or file path")
	sqlCmd.Flags().BoolVarP(&sqlExecute, "execute", "x", false, "Execute the generated SQL query")
	sqlCmd.Flags().StringVarP(&sqlOutput, "output", "o", "", "Output file path for the SQL (default stdout)")
}

func runSQL(cmd *cobra.Command, args []string) error {
	question := strings.Join(args, " ")

	// 1. Resolve DB Connection
	connStr := sqlDbPath
	if connStr == "" {
		// Try to find a default sqlite db
		if _, err := os.Stat("recac.db"); err == nil {
			connStr = "recac.db"
		} else if _, err := os.Stat(".recac.db"); err == nil {
			connStr = ".recac.db"
		} else {
			return fmt.Errorf("connection string or file path required (use --db)")
		}
	}

	// 2. Extract Schema
	// reuse extractSchema from schema.go
	schema, err := extractSchema(connStr)
	if err != nil {
		return fmt.Errorf("failed to extract schema: %w", err)
	}

	// 3. Prepare AI Prompt
	ddl := schemaToDDL(schema)

	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get cwd: %w", err)
	}

	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-sql")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	prompt := fmt.Sprintf(`You are an expert SQL Data Analyst.
Given the following database schema, write a SQL query to answer the question.
Return ONLY the raw SQL query. Do not use Markdown formatting (no code blocks).

Schema:
%s

Question:
%s
`, ddl, question)

	fmt.Fprintln(cmd.ErrOrStderr(), "🤖 Generating SQL...")

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	sqlQuery := utils.CleanCodeBlock(resp)

	// 4. Output SQL
	if sqlOutput != "" {
		if err := os.WriteFile(sqlOutput, []byte(sqlQuery), 0644); err != nil {
			return fmt.Errorf("failed to write output: %w", err)
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "✅ SQL saved to %s\n", sqlOutput)
	} else {
		// If not executing, print to stdout. If executing, print to stderr to keep stdout for results?
		// Or print "Generated SQL:"
		fmt.Fprintf(cmd.OutOrStdout(), "-- Generated SQL:\n%s\n\n", sqlQuery)
	}

	// 5. Execute (if requested)
	if sqlExecute {
		return executeSQL(cmd, connStr, sqlQuery)
	}

	return nil
}

func schemaToDDL(schema *DatabaseSchema) string {
	var sb strings.Builder
	for _, t := range schema.Tables {
		sb.WriteString(fmt.Sprintf("TABLE %s (\n", t.Name))
		for _, c := range t.Columns {
			sb.WriteString(fmt.Sprintf("  %s %s", c.Name, c.Type))
			if c.PK {
				sb.WriteString(" PRIMARY KEY")
			}
			sb.WriteString(",\n")
		}
		for _, fk := range t.ForeignKeys {
			sb.WriteString(fmt.Sprintf("  FOREIGN KEY (%s) REFERENCES %s(%s),\n", fk.FromColumn, fk.ToTable, fk.ToColumn))
		}
		sb.WriteString(");\n")
	}
	return sb.String()
}

func executeSQL(cmd *cobra.Command, connStr, query string) error {
	var dbType string
	var dsn string

	if strings.HasPrefix(connStr, "postgres://") || strings.HasPrefix(connStr, "postgresql://") {
		dbType = "postgres"
		dsn = connStr
	} else {
		dbType = "sqlite"
		dsn = connStr
	}

	db, err := sql.Open(dbType, dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	rows, err := db.Query(query)
	if err != nil {
		return fmt.Errorf("query execution failed: %w", err)
	}
	defer rows.Close()

	// Get columns
	cols, err := rows.Columns()
	if err != nil {
		return err
	}

	// Prepare tabwriter
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)

	// Header
	for i, col := range cols {
		fmt.Fprint(w, col)
		if i < len(cols)-1 {
			fmt.Fprint(w, "\t")
		}
	}
	fmt.Fprint(w, "\n")

	// Separator
	for i := range cols {
		fmt.Fprint(w, strings.Repeat("-", len(cols[i])))
		if i < len(cols)-1 {
			fmt.Fprint(w, "\t")
		}
	}
	fmt.Fprint(w, "\n")

	// Values
	values := make([]interface{}, len(cols))
	valuePtrs := make([]interface{}, len(cols))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	for rows.Next() {
		if err := rows.Scan(valuePtrs...); err != nil {
			return err
		}
		for i, val := range values {
			var v string
			if val == nil {
				v = "NULL"
			} else {
				switch t := val.(type) {
				case []byte:
					v = string(t)
				default:
					v = fmt.Sprintf("%v", t)
				}
			}
			fmt.Fprint(w, v)
			if i < len(cols)-1 {
				fmt.Fprint(w, "\t")
			}
		}
		fmt.Fprint(w, "\n")
	}

	w.Flush()
	return nil
}
