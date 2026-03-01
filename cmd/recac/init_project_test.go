package main

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInitProject_SendStream(t *testing.T) {
	mockAgent := &CLIMockAgent{}

	chunks := []string{}
	callback := func(s string) {
		chunks = append(chunks, s)
	}

	resp, err := mockAgent.SendStream(context.Background(), "prompt", callback)

	assert.NoError(t, err)
	assert.Equal(t, `{ "project_name": "Mock Project", "features": [] }`, resp)
	assert.Len(t, chunks, 1)
	assert.Equal(t, `{ "project_name": "Mock Project", "features": [] }`, chunks[0])
}
