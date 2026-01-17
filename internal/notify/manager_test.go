package notify

import (
	"context"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestManager_Config(t *testing.T) {
	// Setup Viper defaults for test
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	viper.Set("notifications.slack.enabled", true)
	viper.Set("notifications.discord.enabled", false)
	viper.Set("notifications.slack.channel", "#test-channel")

	// Create Manager
	// We can't easily assert that slack client is created without inspecting private fields,
	// but we can verify no panic and basic initialization logic.
	m := NewManager(nil)
	assert.NotNil(t, m)

	// Test isProviderEnabled
	assert.True(t, m.isProviderEnabled("slack"))
	assert.False(t, m.isProviderEnabled("discord"))
}

func TestManager_IsEnabled(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })

	viper.Set("notifications.slack.enabled", true)
	viper.Set("notifications.slack.events.on_start", true)
	viper.Set("notifications.slack.events.on_failure", false)

	m := NewManager(nil)

	assert.True(t, m.isEnabled(EventStart))
	assert.False(t, m.isEnabled(EventFailure))
	// Default behavior for undefined events might depend on implementation,
	// currently code checks "notifications.slack.events." + eventType.
	// If not set, it's false by default in Viper.
	assert.False(t, m.isEnabled(EventSuccess))
}

func TestManager_ThreadState(t *testing.T) {
	// Test Parsing
	jsonState := `{"slack_ts":"123.456","discord_id":"789"}`
	ts := parseThreadState(jsonState)
	assert.Equal(t, "123.456", ts.SlackTS)
	assert.Equal(t, "789", ts.DiscordID)

	legacyState := "123.456"
	tsLegacy := parseThreadState(legacyState)
	assert.Equal(t, "123.456", tsLegacy.SlackTS)
	assert.Empty(t, tsLegacy.DiscordID)

	emptyState := ""
	tsEmpty := parseThreadState(emptyState)
	assert.Empty(t, tsEmpty.SlackTS)

	// Test Dumping
	tsOut := ThreadState{SlackTS: "111", DiscordID: "222"}
	out := dumpThreadState(tsOut)
	assert.Contains(t, out, `"slack_ts":"111"`)
	assert.Contains(t, out, `"discord_id":"222"`)

	// Optimization check: Only Slack
	tsSlackOnly := ThreadState{SlackTS: "111"}
	outSlack := dumpThreadState(tsSlackOnly)
	assert.Equal(t, "111", outSlack)
}

func TestManager_Notify_Disabled(t *testing.T) {
	viper.Reset()
	t.Cleanup(func() { viper.Reset() })
	viper.Set("notifications.slack.enabled", false)
	viper.Set("notifications.discord.enabled", false)

	m := NewManager(nil)
	ctx := context.Background()

	// Should return empty string and no error
	state, err := m.Notify(ctx, EventStart, "test message", "")
	assert.NoError(t, err)
	assert.Empty(t, state)
}

func TestManager_GetStyle(t *testing.T) {
	title, color := getStyle(EventStart)
	assert.NotEmpty(t, title)
	assert.Equal(t, "#3498db", color)

	title, color = getStyle(EventFailure)
	assert.NotEmpty(t, title)
	assert.Equal(t, "#a30200", color)

	title, color = getStyle("unknown_event")
	assert.Equal(t, "📢 Notification", title)
	assert.Equal(t, "#808080", color)
}
