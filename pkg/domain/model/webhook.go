package model

import "time"

// WebhookEventType represents the type of webhook event received
type WebhookEventType string

const (
	EventTypePullRequest WebhookEventType = "pull_request"
	EventTypeRelease     WebhookEventType = "release"
	EventTypeUnknown     WebhookEventType = "unknown"
)

// WebhookEvent represents a webhook event received from GitHub
type WebhookEvent struct {
	ID         string           // Retrieved from X-GitHub-Delivery header
	Type       WebhookEventType // Retrieved from X-GitHub-Event header
	Action     string           // Event action (e.g., opened, released)
	Repository string           // Repository name
	Sender     string           // Sender username
	ReceivedAt time.Time        // Time when the event was received
	RawPayload []byte           // Raw JSON payload
}

// IsSupportedEvent checks if the event is supported
func (e *WebhookEvent) IsSupportedEvent() bool {
	switch e.Type {
	case EventTypePullRequest:
		return e.Action == "opened"
	case EventTypeRelease:
		return e.Action == "released"
	default:
		return false
	}
}
