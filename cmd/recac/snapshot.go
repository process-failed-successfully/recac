package main

import (
	"fmt"
	"recac/internal/git"
	"recac/internal/snapshot"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Manage session snapshots",
	Long:  `Create, list, restore, and delete session snapshots. Snapshots capture the git state and agent memory/DB.`,
}

var snapshotSaveCmd = &cobra.Command{
	Use:   "save [name]",
	Short: "Save a snapshot",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		description, _ := cmd.Flags().GetString("description")

		workspace, err := getWorkspaceForSnapshot(cmd)
		if err != nil {
			return err
		}

		gitClient := gitClientFactory()
		client, ok := gitClient.(git.IClient)
		if !ok {
			return fmt.Errorf("git client mismatch: expected internal/git.IClient")
		}
		mgr := snapshot.NewManager(workspace, client)

		if err := mgr.Save(name, description); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Snapshot '%s' saved successfully.\n", name)
		return nil
	},
}

var snapshotRestoreCmd = &cobra.Command{
	Use:   "restore [name]",
	Short: "Restore a snapshot",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		workspace, err := getWorkspaceForSnapshot(cmd)
		if err != nil {
			return err
		}

		gitClient := gitClientFactory()
		client, ok := gitClient.(git.IClient)
		if !ok {
			return fmt.Errorf("git client mismatch: expected internal/git.IClient")
		}
		mgr := snapshot.NewManager(workspace, client)

		if err := mgr.Restore(name); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Snapshot '%s' restored successfully.\n", name)
		return nil
	},
}

var snapshotListCmd = &cobra.Command{
	Use:   "list",
	Short: "List snapshots",
	RunE: func(cmd *cobra.Command, args []string) error {
		workspace, err := getWorkspaceForSnapshot(cmd)
		if err != nil {
			return err
		}

		gitClient := gitClientFactory()
		client, ok := gitClient.(git.IClient)
		if !ok {
			return fmt.Errorf("git client mismatch: expected internal/git.IClient")
		}
		mgr := snapshot.NewManager(workspace, client)

		snapshots, err := mgr.List()
		if err != nil {
			return err
		}

		if len(snapshots) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No snapshots found.")
			return nil
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "NAME\tTIME\tCOMMIT\tDESCRIPTION")
		for _, s := range snapshots {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				s.Name,
				s.Timestamp.Format(time.RFC822),
				s.CommitSHA[:7],
				s.Description,
			)
		}
		w.Flush()
		return nil
	},
}

var snapshotDeleteCmd = &cobra.Command{
	Use:   "delete [name]",
	Short: "Delete a snapshot",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		workspace, err := getWorkspaceForSnapshot(cmd)
		if err != nil {
			return err
		}

		gitClient := gitClientFactory()
		client, ok := gitClient.(git.IClient)
		if !ok {
			return fmt.Errorf("git client mismatch: expected internal/git.IClient")
		}
		mgr := snapshot.NewManager(workspace, client)

		if err := mgr.Delete(name); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Snapshot '%s' deleted successfully.\n", name)
		return nil
	},
}

func init() {
	snapshotCmd.PersistentFlags().StringP("session", "s", "", "Target session name")
	snapshotSaveCmd.Flags().StringP("description", "m", "", "Description of the snapshot")

	snapshotCmd.AddCommand(snapshotSaveCmd)
	snapshotCmd.AddCommand(snapshotRestoreCmd)
	snapshotCmd.AddCommand(snapshotListCmd)
	snapshotCmd.AddCommand(snapshotDeleteCmd)

	rootCmd.AddCommand(snapshotCmd)
}

func getWorkspaceForSnapshot(cmd *cobra.Command) (string, error) {
	sessionName, _ := cmd.Flags().GetString("session")
	var resolveArgs []string
	if sessionName != "" {
		resolveArgs = []string{sessionName}
	}
	return resolveWorkspace(cmd, resolveArgs)
}
