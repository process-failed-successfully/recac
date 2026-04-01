package orchestrator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestUpdatePollInterval(t *testing.T) {
	orch := New(&MockPoller{}, &MockSpawner{}, 5*time.Second)

	assert.Equal(t, 5*time.Second, orch.PollInterval)

	orch.UpdatePollInterval(10 * time.Second)

	assert.Equal(t, 10*time.Second, orch.PollInterval)

	// verify channel
	select {
	case updated := <-orch.updateIntervalCh:
		assert.Equal(t, 10*time.Second, updated)
	default:
		t.Fatal("expected updateIntervalCh to have value")
	}

    // test updating again when channel is full
	orch.UpdatePollInterval(15 * time.Second)
	assert.Equal(t, 15*time.Second, orch.PollInterval)
}
