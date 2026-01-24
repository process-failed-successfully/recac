package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Manage workspace snapshots",
	Long:  `Create, list, restore, and delete snapshots of the current workspace state (Git + Agent Memory).`,
}

var snapshotCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new snapshot",
	Args:  cobra.ExactArgs(1),
	RunE:  runSnapshotCreate,
}

var snapshotListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available snapshots",
	RunE:  runSnapshotList,
}

var snapshotRestoreCmd = &cobra.Command{
	Use:   "restore [name]",
	Short: "Restore a snapshot",
	Args:  cobra.ExactArgs(1),
	RunE:  runSnapshotRestore,
}

var snapshotDeleteCmd = &cobra.Command{
	Use:   "delete [name]",
	Short: "Delete a snapshot",
	Args:  cobra.ExactArgs(1),
	RunE:  runSnapshotDelete,
}

func init() {
	rootCmd.AddCommand(snapshotCmd)
	snapshotCmd.AddCommand(snapshotCreateCmd)
	snapshotCmd.AddCommand(snapshotListCmd)
	snapshotCmd.AddCommand(snapshotRestoreCmd)
	snapshotCmd.AddCommand(snapshotDeleteCmd)
}

func runSnapshotCreate(cmd *cobra.Command, args []string) error {
	name := args[0]
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	client := gitClientFactory()
	if !client.RepoExists(cwd) {
		return fmt.Errorf("current directory is not a git repository")
	}

	// 1. Create Git Tag
	tagName := "snapshot/" + name
	// Check if tag exists? Git will fail if it does.
	if err := client.Tag(cwd, tagName); err != nil {
		return fmt.Errorf("failed to create git tag: %w", err)
	}

	// 2. Backup Agent State
	snapshotDir := filepath.Join(cwd, ".recac", "snapshots", name)
	if err := os.MkdirAll(snapshotDir, 0755); err != nil {
		return fmt.Errorf("failed to create snapshot directory: %w", err)
	}

	filesToBackup := []string{".agent_state.json", ".recac.db"}
	for _, filename := range filesToBackup {
		src := filepath.Join(cwd, filename)
		dst := filepath.Join(snapshotDir, filename)
		if _, err := os.Stat(src); err == nil {
			if err := copyFile(src, dst); err != nil {
				return fmt.Errorf("failed to backup %s: %w", filename, err)
			}
		}
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✅ Snapshot '%s' created.\n", name)
	return nil
}

func runSnapshotList(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	snapshotsDir := filepath.Join(cwd, ".recac", "snapshots")
	entries, err := os.ReadDir(snapshotsDir)
	if err != nil {
		if os.IsNotExist(err) {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "No snapshots found.")
			return nil
		}
		return err
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	_, _ = fmt.Fprintln(w, "NAME\tCREATED\tTAG")

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		info, err := entry.Info()
		if err != nil {
			continue
		}

		_, _ = fmt.Fprintf(w, "%s\t%s\tsnapshot/%s\n", name, info.ModTime().Format(time.RFC822), name)
	}
	w.Flush()
	return nil
}

func runSnapshotRestore(cmd *cobra.Command, args []string) error {
	name := args[0]
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	snapshotDir := filepath.Join(cwd, ".recac", "snapshots", name)
	if _, err := os.Stat(snapshotDir); os.IsNotExist(err) {
		return fmt.Errorf("snapshot '%s' not found", name)
	}

	client := gitClientFactory()
	tagName := "snapshot/" + name

	// 1. Git Checkout
	if err := client.Checkout(cwd, tagName); err != nil {
		return fmt.Errorf("failed to checkout tag %s: %w", tagName, err)
	}

	// 2. Restore Agent State
	filesToRestore := []string{".agent_state.json", ".recac.db"}
	for _, filename := range filesToRestore {
		src := filepath.Join(snapshotDir, filename)
		dst := filepath.Join(cwd, filename)
		if _, err := os.Stat(src); err == nil {
			if err := copyFile(src, dst); err != nil {
				return fmt.Errorf("failed to restore %s: %w", filename, err)
			}
		} else {
			// If file was not in snapshot, remove it from CWD if it exists?
			// Maybe safer to leave it or warn?
			// Logic: Restore state to EXACTLY what it was. If file didn't exist then, it shouldn't exist now?
			// But deleting user files is risky.
			// Let's just overwrite if exists in snapshot.
		}
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✅ Snapshot '%s' restored.\n", name)
	return nil
}

func runSnapshotDelete(cmd *cobra.Command, args []string) error {
	name := args[0]
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	client := gitClientFactory()
	tagName := "snapshot/" + name

	// 1. Delete Git Tag
	// Ignore error if tag doesn't exist?
	if err := client.DeleteTag(cwd, tagName); err != nil {
		// Log warning but continue cleanup
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Warning: failed to delete tag %s: %v\n", tagName, err)
	}

	// 2. Delete Snapshot Dir
	snapshotDir := filepath.Join(cwd, ".recac", "snapshots", name)
	if err := os.RemoveAll(snapshotDir); err != nil {
		return fmt.Errorf("failed to remove snapshot files: %w", err)
	}

	_, _ = fmt.Fprintf(cmd.OutOrStdout(), "✅ Snapshot '%s' deleted.\n", name)
	return nil
}

func copyFile(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		closeErr := in.Close()
		if err == nil {
			err = closeErr
		}
	}()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		closeErr := out.Close()
		if err == nil {
			err = closeErr
		}
	}()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
