package main

import (
	"fmt"
	"recac/internal/db"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var featureAddCmd = &cobra.Command{
	Use:   "add [description]",
	Short: "Add a new feature/task",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		description := strings.Join(args, " ")
		fl, err := loadFeatures()
		if err != nil {
			return err
		}

		// Generate ID based on max existing numeric suffix
		maxID := 0
		re := regexp.MustCompile(`task-(\d+)`)
		for _, f := range fl.Features {
			matches := re.FindStringSubmatch(f.ID)
			if len(matches) > 1 {
				if num, err := strconv.Atoi(matches[1]); err == nil {
					if num > maxID {
						maxID = num
					}
				}
			}
		}
		id := fmt.Sprintf("task-%d", maxID+1)

		newFeature := db.Feature{
			ID:          id,
			Category:    "Task",
			Priority:    "Medium",
			Description: description,
			Status:      "todo",
			Passes:      false,
			Steps:       []string{}, // Can be populated by agent later
		}

		fl.Features = append(fl.Features, newFeature)

		if err := saveFeatures(fl); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Added task: %s\n", id)
		return nil
	},
}

func init() {
	featureCmd.AddCommand(featureAddCmd)
}
