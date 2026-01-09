package scenarios

import (
	"testing"
)

func TestRegistry_ContainsPrimePython(t *testing.T) {
	// init() functions should have run
	if _, ok := Registry["prime-python"]; !ok {
		t.Error("Registry does not contain 'prime-python' scenario")
	}
	if _, ok := Registry["simple-readme"]; !ok {
		t.Error("Registry does not contain 'simple-readme' scenario")
	}
}
