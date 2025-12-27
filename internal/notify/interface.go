package notify

import "context"

// Notifier defines the interface for sending notifications.
type Notifier interface {
	Notify(ctx context.Context, message string) error
}
