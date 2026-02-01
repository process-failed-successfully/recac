package prompts

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestListPrompts(t *testing.T) {
	prompts, err := ListPrompts()
	assert.NoError(t, err)
	assert.NotEmpty(t, prompts)

	expected := []string{
		Planner,
		ManagerReview,
		CodingAgent,
		Initializer,
		QAAgent,
		TPMAgent,
		ArchitectAgent,
	}

	for _, p := range expected {
		assert.Contains(t, prompts, p, "Expected prompt %s to be in list", p)
	}
}
