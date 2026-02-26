package main

import (
	"fmt"
	"os"
	"recac/internal/undo"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var undoCmd = &cobra.Command{
	Use:   "undo [operation-id]",
	Short: "Undo recent file changes",
	Long: `Reverts changes made by recac commands (like refactor, heal, etc).
If no operation ID is provided, it reverts the most recent operation.`,
	RunE: runUndo,
}

var undoListCmd = &cobra.Command{
	Use:   "list",
	Short: "List undo history",
	RunE:  runUndoList,
}

func init() {
	rootCmd.AddCommand(undoCmd)
	undoCmd.AddCommand(undoListCmd)
}

func runUndo(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	m := undo.NewManager(cwd)

	var opID string
	if len(args) > 0 {
		opID = args[0]
	} else {
		// Get most recent
		ops, err := m.List()
		if err != nil {
			return fmt.Errorf("failed to list operations: %w", err)
		}
		if len(ops) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "Nothing to undo.")
			return nil
		}
		opID = ops[0].ID
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Undoing operation %s...\n", opID)
	if err := m.Restore(opID); err != nil {
		return fmt.Errorf("undo failed: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "✅ Undo successful.")
	return nil
}

func runUndoList(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	m := undo.NewManager(cwd)

	ops, err := m.List()
	if err != nil {
		return fmt.Errorf("failed to list operations: %w", err)
	}

	if len(ops) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No undo history.")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tTIME\tFILES")
	for _, op := range ops {
		fileCount := len(op.Changes)
		fileSummary := ""
		if fileCount > 0 {
			fileSummary = fmt.Sprintf("%s (+%d more)", op.Changes[0].Path, fileCount-1)
			if fileCount == 1 {
				fileSummary = op.Changes[0].Path
			}
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", op.ID, op.Timestamp.Format(time.RFC822), fileSummary)
	}
	w.Flush()
	return nil
}
