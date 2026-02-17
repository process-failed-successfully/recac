package notify

import (
	"context"
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestInitDiscord_Enabled(t *testing.T) {
	// Setup
	viper.Set("notifications.discord.enabled", true)
	os.Setenv("DISCORD_BOT_TOKEN", "fake-token")
	os.Setenv("DISCORD_CHANNEL_ID", "123456")
	defer func() {
		viper.Reset()
		os.Unsetenv("DISCORD_BOT_TOKEN")
		os.Unsetenv("DISCORD_CHANNEL_ID")
	}()

	m := &Manager{}
	m.initDiscord()

	assert.NotNil(t, m.discordNotifier)
}

func TestInitDiscord_Disabled(t *testing.T) {
	viper.Set("notifications.discord.enabled", false)

	m := &Manager{}
	m.initDiscord()

	assert.Nil(t, m.discordNotifier)
}

func TestInitDiscord_MissingToken(t *testing.T) {
	viper.Set("notifications.discord.enabled", true)
	os.Unsetenv("DISCORD_BOT_TOKEN")

	m := &Manager{}
	m.initDiscord()

	assert.Nil(t, m.discordNotifier)
}

func TestStart_SocketClient(t *testing.T) {
	m := &Manager{} // No socketClient
	// Should not panic
	m.Start(context.Background())
}
