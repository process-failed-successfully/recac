package db

import (
	"os"
	"testing"
)

func TestSQLiteSignals(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "recac-test-db-*.sqlite")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	store, err := NewSQLiteStore(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// 1. Test SetSignal
	if err := store.SetSignal("COMPLETED", "true"); err != nil {
		t.Errorf("SetSignal failed: %v", err)
	}

	// 2. Test GetSignal
	val, err := store.GetSignal("COMPLETED")
	if err != nil {
		t.Errorf("GetSignal failed: %v", err)
	}
	if val != "true" {
		t.Errorf("Expected 'true', got '%s'", val)
	}

	// 3. Test GetSignal (Not Found)
	val, err = store.GetSignal("NON_EXISTENT")
	if err != nil {
		t.Errorf("GetSignal (NonExistent) failed: %v", err)
	}
	if val != "" {
		t.Errorf("Expected empty string for non-existent key, got '%s'", val)
	}

	// 4. Test Update Signal
	if err := store.SetSignal("COMPLETED", "false"); err != nil {
		t.Errorf("SetSignal (Update) failed: %v", err)
	}
	val, err = store.GetSignal("COMPLETED")
	if err != nil {
		t.Errorf("GetSignal (After Update) failed: %v", err)
	}
	if val != "false" {
		t.Errorf("Expected 'false', got '%s'", val)
	}

	// 5. Test DeleteSignal
	if err := store.DeleteSignal("COMPLETED"); err != nil {
		t.Errorf("DeleteSignal failed: %v", err)
	}
	val, err = store.GetSignal("COMPLETED")
	if err != nil {
		t.Errorf("GetSignal (After Delete) failed: %v", err)
	}
	if val != "" {
		t.Errorf("Expected empty string after delete, got '%s'", val)
	}
}
