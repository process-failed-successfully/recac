package notify

import (
	"context"
	"fmt"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

// SocketClient defines the interface for socket interactions (Ack).
type SocketClient interface {
	Ack(req socketmode.Request, payload ...interface{})
}

// HandleEvents listens for incoming Socket Mode events.
// This is a simplified handler to prove the connection works.
func (m *Manager) HandleEvents(ctx context.Context, events <-chan socketmode.Event, client SocketClient) {
	for {
		select {
		case <-ctx.Done():
			return
		case evt := <-events:
			switch evt.Type {
			case socketmode.EventTypeConnecting:
				if m.logger != nil {
					m.logger("Connecting to Slack Socket Mode...")
				}
			case socketmode.EventTypeConnectionError:
				if m.logger != nil {
					m.logger("Connection failed. Retrying later...")
				}
			case socketmode.EventTypeConnected:
				if m.logger != nil {
					m.logger("Connected to Slack Socket Mode via WebSocket!")
				}
			case socketmode.EventTypeEventsAPI:
				eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
				if !ok {
					continue
				}

				if client != nil && evt.Request != nil {
					client.Ack(*evt.Request)
				}

				switch eventsAPIEvent.Type {
				case slackevents.CallbackEvent:
					innerEvent := eventsAPIEvent.InnerEvent
					switch ev := innerEvent.Data.(type) {
					case *slackevents.AppMentionEvent:
						if m.logger != nil {
							m.logger("Received Mention: %s", ev.Text)
						}
						// Echo back just to prove it works
						m.client.PostMessage(ev.Channel, slack.MsgOptionText(fmt.Sprintf("Yes, hello! I received: %s", ev.Text), false))
					}
				}
			}
		}
	}
}
