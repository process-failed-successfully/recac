package breaking

import (
	"fmt"
	"sort"
)

type ChangeType string

const (
	ChangeRemoved ChangeType = "REMOVED"
	ChangeChanged ChangeType = "CHANGED"
	ChangeAdded   ChangeType = "ADDED"
)

type Change struct {
	Type       ChangeType
	Identifier string
	Message    string
}

// Compare compares the old API with the new API and returns a list of changes.
func Compare(oldAPI, newAPI *API) []Change {
	var changes []Change

	// Check for Removed or Changed
	for id, oldSig := range oldAPI.Identifiers {
		newSig, exists := newAPI.Identifiers[id]
		if !exists {
			changes = append(changes, Change{
				Type:       ChangeRemoved,
				Identifier: id,
				Message:    fmt.Sprintf("Identifier %s was removed", id),
			})
		} else if oldSig != newSig {
			changes = append(changes, Change{
				Type:       ChangeChanged,
				Identifier: id,
				Message:    fmt.Sprintf("Signature changed for %s.\nOld: %s\nNew: %s", id, oldSig, newSig),
			})
		}
	}

	// Check for Added
	for id := range newAPI.Identifiers {
		if _, exists := oldAPI.Identifiers[id]; !exists {
			changes = append(changes, Change{
				Type:       ChangeAdded,
				Identifier: id,
				Message:    fmt.Sprintf("Identifier %s was added", id),
			})
		}
	}

	// Sort changes by Identifier
	sort.Slice(changes, func(i, j int) bool {
		return changes[i].Identifier < changes[j].Identifier
	})

	return changes
}
