package notify

import "context"

// Notifier defines the interface for sending notifications.
// Notifier defines the interface for sending notifications.
type Notifier interface {
	Start(ctx context.Context)
	Notify(ctx context.Context, eventType string, message string, threadTS string) (string, error)
	AddReaction(ctx context.Context, timestamp, reaction string) error
}
