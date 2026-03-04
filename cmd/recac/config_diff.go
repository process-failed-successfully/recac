package main

import (
	"fmt"
	"reflect"
	"sort"
	"text/tabwriter"

	"recac/internal/config"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configDiffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show differences between current configuration and defaults",
	Long: `Display a table of all configuration keys that have been overridden
from their default values.

This is useful for seeing exactly what custom settings you have applied.
Values for sensitive keys are redacted by default.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Define defaults using internal/config package to prevent duplication
		defaults := config.GetDefaultConfig()

		keys := viper.AllKeys()
		sort.Strings(keys)

		type diffEntry struct {
			key          string
			currentValue interface{}
			defaultValue interface{}
		}
		var diffs []diffEntry

		for _, key := range keys {
			currentVal := viper.Get(key)
			defaultVal, hasDefault := defaults[key]

			if !hasDefault {
				// If there's no default and the value is not empty, it's an override.
				// We consider nil, empty string, false, 0 as "empty" for the sake of no-default
				if !isEmpty(currentVal) {
					diffs = append(diffs, diffEntry{key: key, currentValue: currentVal, defaultValue: "<none>"})
				}
				continue
			}

			if !reflect.DeepEqual(currentVal, defaultVal) {
				// viper can sometimes return int as float64, etc. We can convert to string to compare
				currentStr := fmt.Sprintf("%v", currentVal)
				defaultStr := fmt.Sprintf("%v", defaultVal)

				if currentStr != defaultStr {
					diffs = append(diffs, diffEntry{key: key, currentValue: currentVal, defaultValue: defaultVal})
				}
			}
		}

		if len(diffs) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No configuration differences found. You are using the default settings.")
			return nil
		}

		showSensitive, _ := cmd.Flags().GetBool("show-sensitive")

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
		defer w.Flush()

		fmt.Fprintln(w, "KEY\tCURRENT VALUE\tDEFAULT VALUE")
		fmt.Fprintln(w, "---\t-------------\t-------------")

		for _, entry := range diffs {
			currentStr := fmt.Sprintf("%v", entry.currentValue)
			defaultStr := fmt.Sprintf("%v", entry.defaultValue)

			if isSensitive(entry.key) && !showSensitive {
				currentStr = "[REDACTED]"
				if entry.defaultValue != "<none>" {
					defaultStr = "[REDACTED]"
				}
			}

			fmt.Fprintf(w, "%s\t%s\t%s\n", entry.key, currentStr, defaultStr)
		}

		return nil
	},
}

func isEmpty(val interface{}) bool {
	if val == nil {
		return true
	}
	v := reflect.ValueOf(val)
	switch v.Kind() {
	case reflect.String, reflect.Array, reflect.Map, reflect.Slice:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	}
	return false
}

func init() {
	configDiffCmd.Flags().Bool("show-sensitive", false, "Do not redact sensitive values")
}
