package notify

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestNewManager_InitDiscord(t *testing.T) {
	// Setup
	viper.Reset()
	defer viper.Reset()
	os.Clearenv()

	// 1. Disabled
	viper.Set("notifications.discord.enabled", false)
	m := NewManager(nil)
	assert.Nil(t, m.discordNotifier)

	// 2. Enabled but missing token/channel
	viper.Set("notifications.discord.enabled", true)
	m = NewManager(func(format string, args ...interface{}) {}) // Logger to capture warning
	assert.Nil(t, m.discordNotifier)

	// 3. Enabled and configured via Env
	os.Setenv("DISCORD_BOT_TOKEN", "token")
	os.Setenv("DISCORD_CHANNEL_ID", "channel")
	defer os.Unsetenv("DISCORD_BOT_TOKEN")
	defer os.Unsetenv("DISCORD_CHANNEL_ID")

	m = NewManager(nil)
	assert.NotNil(t, m.discordNotifier)

	// 4. Enabled and configured via Viper (Channel)
	os.Unsetenv("DISCORD_CHANNEL_ID")
	viper.Set("notifications.discord.channel", "viper-channel")
	m = NewManager(nil)
	assert.NotNil(t, m.discordNotifier)
}

func TestNewManager_InitSlack(t *testing.T) {
	// Setup
	viper.Reset()
	defer viper.Reset()
	os.Clearenv()

	// 1. Disabled
	viper.Set("notifications.slack.enabled", false)
	m := NewManager(nil)
	assert.Nil(t, m.client)

	// 2. Enabled but missing token
	viper.Set("notifications.slack.enabled", true)
	m = NewManager(func(format string, args ...interface{}) {})
	assert.Nil(t, m.client)

	// 3. Enabled and configured
	os.Setenv("SLACK_BOT_USER_TOKEN", "xoxb-token")
	defer os.Unsetenv("SLACK_BOT_USER_TOKEN")

	m = NewManager(nil)
	assert.NotNil(t, m.client)
	assert.Nil(t, m.socketClient)

	// 4. Socket Mode
	os.Setenv("SLACK_APP_TOKEN", "xapp-token")
	defer os.Unsetenv("SLACK_APP_TOKEN")

	m = NewManager(nil)
	assert.NotNil(t, m.client)
	assert.NotNil(t, m.socketClient)
}

func TestGetStyle_AllEvents(t *testing.T) {
	events := []string{
		EventStart,
		EventSuccess,
		EventFailure,
		EventUserInteraction,
		EventProjectComplete,
		"unknown",
	}

	for _, evt := range events {
		title, color := getStyle(evt)
		assert.NotEmpty(t, title)
		assert.NotEmpty(t, color)
	}
}

func TestParseThreadState_EdgeCases(t *testing.T) {
	ts := parseThreadState(`{"invalid_json"`)
	assert.Equal(t, `{"invalid_json"`, ts.SlackTS)
	assert.Empty(t, ts.DiscordID)

	ts = parseThreadState("")
	assert.Empty(t, ts.SlackTS)
}

func TestManager_Start(t *testing.T) {
	m := &Manager{
		logger: func(format string, args ...interface{}) {},
	}
	// Test safely handling nil socketClient
	m.Start(context.Background())
}

type mockTransport struct {
	RoundTripFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if m.RoundTripFunc != nil {
		return m.RoundTripFunc(req)
	}
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader("")),
	}, nil
}

func TestDiscordNotifier_AddReaction_EmojiMapping(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"white_check_mark", "%E2%9C%85"},
		{":x:", "%E2%9D%8C"},
		{"warning", "%E2%9A%A0%EF%B8%8F"},
		{"other", "other"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			var capturedURL string
			mockTrans := &mockTransport{
				RoundTripFunc: func(req *http.Request) (*http.Response, error) {
					capturedURL = req.URL.String()
					return &http.Response{
						StatusCode: 204,
						Body:       io.NopCloser(strings.NewReader("")),
					}, nil
				},
			}

			notifier := NewDiscordBotNotifier("token", "channel")
			notifier.Client = &http.Client{Transport: mockTrans}

			err := notifier.AddReaction(context.Background(), "msg-123", tt.input)
			assert.NoError(t, err)

			// URL format: .../reactions/{emoji}/@me
			if !strings.HasSuffix(capturedURL, "/reactions/"+tt.expected+"/@me") {
				t.Errorf("URL %s should end with /reactions/%s/@me", capturedURL, tt.expected)
			}
		})
	}
}

func TestDiscordNotifier_Send_Webhook(t *testing.T) {
	var receivedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := new(strings.Builder)
		io.Copy(buf, r.Body)
		receivedBody = buf.String()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier := NewDiscordNotifier(server.URL)
	_, err := notifier.Send(context.Background(), "hello webhook", "")
	assert.NoError(t, err)
	assert.Contains(t, receivedBody, "hello webhook")
}

func TestDiscordNotifier_Send_Error(t *testing.T) {
	// Test error when no config
	notifier := &DiscordNotifier{Client: &http.Client{Timeout: time.Second}}
	_, err := notifier.Send(context.Background(), "msg", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "discord not configured")
}
