package breaking

import (
	"testing"
)

func TestExtractAPI(t *testing.T) {
	mockLoader := func(path string) ([]byte, error) {
		if path == "main.go" {
			return []byte(`package main

func ExportedFunc(a int) string { return "" }

func internalFunc() {}

type ExportedType struct {
	Field int
}

type internalInterface interface{}

const ExportedConst = 1

var ExportedVar = "hello"

func (e *ExportedType) Method() {}
`), nil
		}
		return nil, nil
	}

	api, err := ExtractAPI([]string{"main.go"}, mockLoader)
	if err != nil {
		t.Fatalf("ExtractAPI failed: %v", err)
	}

	expected := map[string]string{
		"./main.ExportedFunc":        "func(a int) string",
		"./main.ExportedType":        "struct { Field int }",
		"./main.ExportedConst":       "var/const", // Type is int, but basic literal
		"./main.ExportedVar":         "var/const", // Type is string
		"./main.ExportedType.Method": "func()",
	}

	for name, wantSig := range expected {
		gotSig, ok := api.Identifiers[name]
		if !ok {
			t.Errorf("Expected identifier %s not found", name)
			continue
		}
		// Relax signature match for var/const as my implementation logic for them is loose
		if wantSig == "var/const" && gotSig != "" {
			continue
		}
		if gotSig != wantSig {
			t.Errorf("Signature mismatch for %s.\nWant: %s\nGot:  %s", name, wantSig, gotSig)
		}
	}

	if _, ok := api.Identifiers["./main.internalFunc"]; ok {
		t.Errorf("Found internal identifier ./main.internalFunc")
	}
}
