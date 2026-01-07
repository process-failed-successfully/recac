package notify

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/slack-go/slack"
	"github.com/spf13/viper"
)

// --- Mocks ---

type mockSlackClient struct {
	mu            sync.Mutex
	postMsgCount  int
	reactionCount int
	postMsgErr    error
	reactionErr   error
	channel       string
	ts            string
}

func (m *mockSlackClient) PostMessageContext(ctx context.Context, channelID string, options ...slack.MsgOption) (string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.postMsgCount++
	return "test-channel", "new-ts", m.postMsgErr
}

func (m *mockSlackClient) PostMessage(channelID string, options ...slack.MsgOption) (string, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.postMsgCount++
	return "test-channel", "new-ts", m.postMsgErr
}

func (m *mockSlackClient) AddReactionContext(ctx context.Context, name string, item slack.ItemRef) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reactionCount++
	return m.reactionErr
}

type mockDiscordNotifier struct {
	mu         sync.Mutex
	sendCount  int
	reactCount int
	sendErr    error
	reactErr   error
}

func (m *mockDiscordNotifier) Send(ctx context.Context, message, threadID string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendCount++
	return "new-discord-id", m.sendErr
}

func (m *mockDiscordNotifier) AddReaction(ctx context.Context, messageID, emoji string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reactCount++
	return m.reactErr
}

// --- Test Setup ---

func setupViper() {
	viper.Reset()
	viper.Set("notifications.slack.enabled", true)
	viper.Set("notifications.discord.enabled", true)
	viper.Set("notifications.slack.events.on_start", true)
	os.Setenv("SLACK_BOT_USER_TOKEN", "fake-token")
	os.Setenv("DISCORD_BOT_TOKEN", "fake-token")
	os.Setenv("DISCORD_CHANNEL_ID", "fake-channel")
}

func TestNewManager(t *testing.T) {
	setupViper()
	m := NewManager(nil)
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.client == nil {
		t.Error("slack client not initialized")
	}
	if m.discordNotifier == nil {
		t.Error("discord notifier not initialized")
	}
}

func TestManager_Notify(t *testing.T) {
	setupViper()
	mockSlack := &mockSlackClient{}
	mockDiscord := &mockDiscordNotifier{}

	m := NewManager(nil)
	// Replace clients with mocks
	m.client = mockSlack
	m.discordNotifier = mockDiscord

	ctx := context.Background()

	t.Run("successful notification", func(t *testing.T) {
		mockSlack.postMsgCount = 0
		mockDiscord.sendCount = 0
		newState, err := m.Notify(ctx, EventStart, "test message", "")
		if err != nil {
			t.Fatalf("Notify() returned an unexpected error: %v", err)
		}

		if mockSlack.postMsgCount != 1 {
			t.Errorf("expected 1 slack message, got %d", mockSlack.postMsgCount)
		}
		if mockDiscord.sendCount != 1 {
			t.Errorf("expected 1 discord message, got %d", mockDiscord.sendCount)
		}

		var ts ThreadState
		if err := json.Unmarshal([]byte(newState), &ts); err != nil {
			t.Fatalf("failed to unmarshal new state: %v", err)
		}
		if ts.SlackTS != "new-ts" {
			t.Errorf("unexpected slack ts: %s", ts.SlackTS)
		}
		if ts.DiscordID != "new-discord-id" {
			t.Errorf("unexpected discord id: %s", ts.DiscordID)
		}
	})

	t.Run("event disabled", func(t *testing.T) {
		viper.Set("notifications.slack.events.on_start", false)
		defer viper.Set("notifications.slack.events.on_start", true)

		mockSlack.postMsgCount = 0
		mockDiscord.sendCount = 0
		_, err := m.Notify(ctx, EventStart, "test message", "")
		if err != nil {
			t.Fatalf("Notify() returned an unexpected error: %v", err)
		}
		if mockSlack.postMsgCount > 0 || mockDiscord.sendCount > 0 {
			t.Error("notification was sent for a disabled event")
		}
	})

	t.Run("provider disabled", func(t *testing.T) {
		viper.Set("notifications.discord.enabled", false)
		defer viper.Set("notifications.discord.enabled", true)
		mockSlack.postMsgCount = 0
		mockDiscord.sendCount = 0
		newState, err := m.Notify(ctx, EventStart, "test message", "")
		if err != nil {
			t.Fatalf("Notify() returned an unexpected error: %v", err)
		}
		if mockSlack.postMsgCount != 1 {
			t.Error("slack message was not sent")
		}
		if mockDiscord.sendCount > 0 {
			t.Error("discord message was sent when disabled")
		}
		if !strings.HasPrefix(newState, "new-ts") {
			t.Errorf("expected plain string state, got %s", newState)
		}
	})
}

func TestManager_AddReaction(t *testing.T) {
	setupViper()
	mockSlack := &mockSlackClient{}
	mockDiscord := &mockDiscordNotifier{}

	m := NewManager(nil)
	m.client = mockSlack
	m.discordNotifier = mockDiscord

	ts := ThreadState{SlackTS: "ts", DiscordID: "id"}
	state, _ := json.Marshal(ts)

	err := m.AddReaction(context.Background(), string(state), "✅")
	if err != nil {
		t.Fatalf("AddReaction() failed: %v", err)
	}
	if mockSlack.reactionCount != 1 {
		t.Error("slack reaction not added")
	}
	if mockDiscord.reactCount != 1 {
		t.Error("discord reaction not added")
	}
}

func TestParseThreadState(t *testing.T) {
	t.Run("json", func(t *testing.T) {
		s := `{"slack_ts": "s1", "discord_id": "d1"}`
		ts := parseThreadState(s)
		if ts.SlackTS != "s1" || ts.DiscordID != "d1" {
			t.Errorf("failed to parse json state: %+v", ts)
		}
	})
	t.Run("legacy slack ts", func(t *testing.T) {
		s := "legacy-ts"
		ts := parseThreadState(s)
		if ts.SlackTS != "legacy-ts" || ts.DiscordID != "" {
			t.Errorf("failed to parse legacy state: %+v", ts)
		}
	})
	t.Run("empty string", func(t *testing.T) {
		ts := parseThreadState("")
		if ts.SlackTS != "" || ts.DiscordID != "" {
			t.Errorf("expected empty state from empty string: %+v", ts)
		}
	})
}

func TestDumpThreadState(t *testing.T) {
	t.Run("both", func(t *testing.T) {
		ts := ThreadState{SlackTS: "s1", DiscordID: "d1"}
		s := dumpThreadState(ts)
		var result ThreadState
		if err := json.Unmarshal([]byte(s), &result); err != nil {
			t.Fatal("failed to unmarshal dumped state")
		}
		if result != ts {
			t.Errorf("dump/parse mismatch: got %+v, want %+v", result, ts)
		}
	})
	t.Run("slack only", func(t *testing.T) {
		ts := ThreadState{SlackTS: "s1"}
		s := dumpThreadState(ts)
		if s != "s1" {
			t.Errorf("expected plain string for slack-only state, got '%s'", s)
		}
	})
}
