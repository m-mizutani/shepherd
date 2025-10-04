package model_test

import (
	"testing"

	"github.com/m-mizutani/shepherd/pkg/domain/model"
)

func TestWebhookEvent_IsSupportedEvent(t *testing.T) {
	tests := []struct {
		name     string
		event    *model.WebhookEvent
		expected bool
	}{
		{
			name: "Pull Request opened - supported",
			event: &model.WebhookEvent{
				Type:   model.EventTypePullRequest,
				Action: "opened",
			},
			expected: true,
		},
		{
			name: "Release released - supported",
			event: &model.WebhookEvent{
				Type:   model.EventTypeRelease,
				Action: "released",
			},
			expected: true,
		},
		{
			name: "Pull Request closed - not supported",
			event: &model.WebhookEvent{
				Type:   model.EventTypePullRequest,
				Action: "closed",
			},
			expected: false,
		},
		{
			name: "Pull Request synchronize - not supported",
			event: &model.WebhookEvent{
				Type:   model.EventTypePullRequest,
				Action: "synchronize",
			},
			expected: false,
		},
		{
			name: "Release created - not supported",
			event: &model.WebhookEvent{
				Type:   model.EventTypeRelease,
				Action: "created",
			},
			expected: false,
		},
		{
			name: "Unknown event type",
			event: &model.WebhookEvent{
				Type:   model.EventTypeUnknown,
				Action: "opened",
			},
			expected: false,
		},
		{
			name: "Push event - supported",
			event: &model.WebhookEvent{
				Type:   model.EventTypePush,
				Action: "",
			},
			expected: true,
		},
		{
			name: "Different event type",
			event: &model.WebhookEvent{
				Type:   model.WebhookEventType("issues"),
				Action: "opened",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.event.IsSupportedEvent()
			if got != tt.expected {
				t.Errorf("IsSupportedEvent() = %v, want %v", got, tt.expected)
			}
		})
	}
}
