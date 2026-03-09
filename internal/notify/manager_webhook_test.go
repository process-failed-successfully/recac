package notify

import (
	"os"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestManager_initWebhook(t *testing.T) {
	tests := []struct {
		name           string
		enabled        bool
		viperURL       string
		viperSecret    string
		envURL         string
		envSecret      string
		expectNotifier bool
		expectLog      bool
	}{
		{
			name:           "disabled",
			enabled:        false,
			expectNotifier: false,
		},
		{
			name:           "enabled from viper",
			enabled:        true,
			viperURL:       "http://webhook",
			viperSecret:    "secret",
			expectNotifier: true,
		},
		{
			name:           "enabled from env",
			enabled:        true,
			envURL:         "http://webhook-env",
			envSecret:      "secret-env",
			expectNotifier: true,
		},
		{
			name:           "enabled but no url",
			enabled:        true,
			expectNotifier: false,
			expectLog:      true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			viper.Reset()
			viper.Set("notifications.webhook.enabled", tc.enabled)
			if tc.viperURL != "" {
				viper.Set("notifications.webhook.url", tc.viperURL)
			}
			if tc.viperSecret != "" {
				viper.Set("notifications.webhook.secret", tc.viperSecret)
			}

			if tc.envURL != "" {
				os.Setenv("RECAC_WEBHOOK_URL", tc.envURL)
				defer os.Unsetenv("RECAC_WEBHOOK_URL")
			}
			if tc.envSecret != "" {
				os.Setenv("RECAC_WEBHOOK_SECRET", tc.envSecret)
				defer os.Unsetenv("RECAC_WEBHOOK_SECRET")
			}

			logCalled := false
			logger := func(msg string, args ...interface{}) {
				logCalled = true
			}

			m := &Manager{logger: logger}
			m.initWebhook()

			if tc.expectNotifier {
				assert.NotNil(t, m.webhookNotifier)
			} else {
				assert.Nil(t, m.webhookNotifier)
			}

			if tc.expectLog {
				assert.True(t, logCalled)
			}
		})
	}
}
