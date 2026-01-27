package notify

import (
	"context"
	"fmt"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

// HandleEvents listens for incoming Socket Mode events.
// This is a simplified handler to prove the connection works.
func (m *Manager) HandleEvents(ctx context.Context) {
	if m.socketClient == nil {
		return
	}
	m.handleEventsLoop(ctx, m.socketClient.Events, func(req socketmode.Request) {
		m.socketClient.Ack(req)
	})
}

func (m *Manager) handleEventsLoop(ctx context.Context, events <-chan socketmode.Event, ackFunc func(socketmode.Request)) {
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
				if evt.Request != nil {
					ackFunc(*evt.Request)
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
						if m.client != nil {
							m.client.PostMessage(ev.Channel, slack.MsgOptionText(fmt.Sprintf("Yes, hello! I received: %s", ev.Text), false))
						}
					}
				}
			}
		}
	}
}
