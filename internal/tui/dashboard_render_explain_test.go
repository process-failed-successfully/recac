package tui

import (
	"testing"
	"github.com/stretchr/testify/assert"
)

func TestRenderExplain(t *testing.T) {
	input := "# Hello\n\nThis is a test."
	out := renderExplain(input)

	assert.NotEmpty(t, out)
	assert.NotEqual(t, input, out) // should be styled by glamour
}
