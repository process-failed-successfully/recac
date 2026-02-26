package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// -- Data Structures --

type KeyResult struct {
	ID          string  `json:"id"`
	Description string  `json:"description"`
	Current     float64 `json:"current"`
	Target      float64 `json:"target"`
	Unit        string  `json:"unit"`
}

type Objective struct {
	ID          string      `json:"id"`
	Description string      `json:"description"`
	KeyResults  []KeyResult `json:"key_results"`
}

type OKRData struct {
	Objectives []Objective `json:"objectives"`
}

var okrFile = ".recac/okrs.json"

// -- Command Setup --

var okrCmd = &cobra.Command{
	Use:   "okr",
	Short: "Manage Objectives and Key Results (OKRs)",
	Long:  `Manage your project's OKRs. Create objectives, add key results, and track progress.`,
}

// -- Subcommands --

var okrInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize an empty OKR file",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := os.Stat(okrFile); err == nil {
			return fmt.Errorf("OKR file already exists at %s", okrFile)
		}
		if err := os.MkdirAll(filepath.Dir(okrFile), 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
		empty := OKRData{Objectives: []Objective{}}
		return saveOKRs(okrFile, empty)
	},
}

var okrListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all OKRs",
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := loadOKRs(okrFile)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintln(cmd.ErrOrStderr(), "No OKRs found. Run 'recac okr init' to start.")
				return nil
			}
			return err
		}

		if len(data.Objectives) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No objectives defined.")
			return nil
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		for _, o := range data.Objectives {
			fmt.Fprintf(w, "OBJECTIVE %s:\t%s\n", o.ID, o.Description)
			if len(o.KeyResults) == 0 {
				fmt.Fprintf(w, "  (No Key Results)\n")
			} else {
				for _, kr := range o.KeyResults {
					progress := 0.0
					if kr.Target != 0 {
						progress = (kr.Current / kr.Target) * 100
					}
					fmt.Fprintf(w, "  KR %s:\t%s\t[%.1f%%] (%.1f/%.1f %s)\n", kr.ID, kr.Description, progress, kr.Current, kr.Target, kr.Unit)
				}
			}
			fmt.Fprintln(w, "")
		}
		w.Flush()
		return nil
	},
}

var okrAddObjCmd = &cobra.Command{
	Use:   "add [description]",
	Short: "Add a new Objective",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		desc := args[0]
		data, err := loadOKRs(okrFile)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if os.IsNotExist(err) {
			// Auto-init for better DX
			if err := os.MkdirAll(filepath.Dir(okrFile), 0755); err != nil {
				return err
			}
			data = OKRData{Objectives: []Objective{}}
		}

		id := fmt.Sprintf("O%d", len(data.Objectives)+1)
		newObj := Objective{
			ID:          id,
			Description: desc,
			KeyResults:  []KeyResult{},
		}
		data.Objectives = append(data.Objectives, newObj)

		if err := saveOKRs(okrFile, data); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Added Objective %s: %s\n", id, desc)
		return nil
	},
}

var (
	krObjectiveID string
	krTarget      float64
	krUnit        string
	krCurrent     float64
)

var okrAddKRCmd = &cobra.Command{
	Use:   "kr [description]",
	Short: "Add a Key Result to an Objective",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		desc := args[0]
		if krObjectiveID == "" {
			return errors.New("objective ID is required (--objective)")
		}
		if krTarget == 0 {
			return errors.New("target value is required (--target)")
		}

		data, err := loadOKRs(okrFile)
		if err != nil {
			return err
		}

		objIndex := -1
		for i, o := range data.Objectives {
			if strings.EqualFold(o.ID, krObjectiveID) {
				objIndex = i
				break
			}
		}
		if objIndex == -1 {
			return fmt.Errorf("objective '%s' not found", krObjectiveID)
		}

		krID := fmt.Sprintf("%s-KR%d", data.Objectives[objIndex].ID, len(data.Objectives[objIndex].KeyResults)+1)
		newKR := KeyResult{
			ID:          krID,
			Description: desc,
			Target:      krTarget,
			Current:     krCurrent, // Optional initial value
			Unit:        krUnit,
		}
		data.Objectives[objIndex].KeyResults = append(data.Objectives[objIndex].KeyResults, newKR)

		if err := saveOKRs(okrFile, data); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Added KR %s to %s\n", krID, data.Objectives[objIndex].ID)
		return nil
	},
}

var (
	updateKRID    string
	updateCurrent float64
)

var okrUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update progress of a Key Result",
	RunE: func(cmd *cobra.Command, args []string) error {
		if updateKRID == "" {
			return errors.New("KR ID is required (--kr)")
		}

		data, err := loadOKRs(okrFile)
		if err != nil {
			return err
		}

		found := false
		for i := range data.Objectives {
			for j := range data.Objectives[i].KeyResults {
				if strings.EqualFold(data.Objectives[i].KeyResults[j].ID, updateKRID) {
					data.Objectives[i].KeyResults[j].Current = updateCurrent
					found = true
					fmt.Fprintf(cmd.OutOrStdout(), "Updated %s to %.2f\n", updateKRID, updateCurrent)
					break
				}
			}
			if found {
				break
			}
		}

		if !found {
			return fmt.Errorf("KR '%s' not found", updateKRID)
		}

		return saveOKRs(okrFile, data)
	},
}

// -- Helpers --

func loadOKRs(path string) (OKRData, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return OKRData{}, err
	}
	var okrData OKRData
	if err := json.Unmarshal(data, &okrData); err != nil {
		return OKRData{}, err
	}
	return okrData, nil
}

func saveOKRs(path string, data OKRData) error {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, bytes, 0644)
}

func init() {
	rootCmd.AddCommand(okrCmd)
	okrCmd.AddCommand(okrInitCmd)
	okrCmd.AddCommand(okrListCmd)
	okrCmd.AddCommand(okrAddObjCmd)
	okrCmd.AddCommand(okrAddKRCmd)
	okrCmd.AddCommand(okrUpdateCmd)

	okrAddKRCmd.Flags().StringVarP(&krObjectiveID, "objective", "o", "", "ID of the objective (e.g., O1)")
	okrAddKRCmd.Flags().Float64VarP(&krTarget, "target", "t", 0, "Target value")
	okrAddKRCmd.Flags().StringVarP(&krUnit, "unit", "u", "", "Unit of measurement (e.g., %, users, ms)")
	okrAddKRCmd.Flags().Float64Var(&krCurrent, "current", 0, "Initial current value")

	okrUpdateCmd.Flags().StringVarP(&updateKRID, "kr", "k", "", "ID of the key result (e.g., O1-KR1)")
	okrUpdateCmd.Flags().Float64VarP(&updateCurrent, "current", "c", 0, "New current value")
}
