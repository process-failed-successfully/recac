package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var featureDoneCmd = &cobra.Command{
	Use:   "done [id]",
	Short: "Mark a task as done",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := updateFeatureStatus(args[0], "done", true); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Updated task %s to done\n", args[0])
		return nil
	},
}

var featurePendingCmd = &cobra.Command{
	Use:   "pending [id]",
	Short: "Mark a task as pending/todo",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := updateFeatureStatus(args[0], "todo", false); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Updated task %s to todo\n", args[0])
		return nil
	},
}

func init() {
	featureCmd.AddCommand(featureDoneCmd)
	featureCmd.AddCommand(featurePendingCmd)
}

func updateFeatureStatus(id, status string, passes bool) error {
	fl, err := loadFeatures()
	if err != nil {
		return err
	}

	found := false
	for i, f := range fl.Features {
		if f.ID == id {
			fl.Features[i].Status = status
			fl.Features[i].Passes = passes
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("task not found: %s", id)
	}

	return saveFeatures(fl)
}
