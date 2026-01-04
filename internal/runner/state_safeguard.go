package runner

import (
	"encoding/json"
	"fmt"
	"os"
)

// LoadSafeguardedState attempts to load the agent state.
// If loading fails due to corruption or invalid JSON, it DELETES the file and returns a fresh state.
// This is a "hard guardrail" to prevent the agent from getting stuck.
func LoadSafeguardedState(path string, target interface{}) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No state file yet, acceptable
		}
		return err
	}
	defer f.Close()

	decoder := json.NewDecoder(f)
	if err := decoder.Decode(target); err != nil {
		// Guardrail activation: Corrupt state detected
		f.Close() // Close before removing

		fmt.Printf("GUARDRAIL: Detected corrupt agent state at %s. Error: %v. \nRemoving file to recover...\n", path, err)

		if rmErr := os.Remove(path); rmErr != nil {
			return fmt.Errorf("failed to remove corrupt state file: %v (original error: %v)", rmErr, err)
		}

		// Return nil so the caller proceeds with empty/default state
		return nil
	}

	return nil
}
