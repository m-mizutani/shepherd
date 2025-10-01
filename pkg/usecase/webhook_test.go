package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/usecase"
)

func TestWebhookUseCase_ProcessEvent(t *testing.T) {
	tests := []struct {
		name    string
		event   *model.WebhookEvent
		wantErr bool
	}{
		{
			name: "Process supported Pull Request event",
			event: &model.WebhookEvent{
				ID:         "test-delivery-1",
				Type:       model.EventTypePullRequest,
				Action:     "opened",
				Repository: "test/repo",
				Sender:     "testuser",
				ReceivedAt: time.Now(),
				RawPayload: []byte(`{"action":"opened"}`),
			},
			wantErr: false,
		},
		{
			name: "Process supported Release event",
			event: &model.WebhookEvent{
				ID:         "test-delivery-2",
				Type:       model.EventTypeRelease,
				Action:     "released",
				Repository: "test/repo",
				Sender:     "testuser",
				ReceivedAt: time.Now(),
				RawPayload: []byte(`{"action":"released"}`),
			},
			wantErr: false,
		},
		{
			name: "Process unsupported event",
			event: &model.WebhookEvent{
				ID:         "test-delivery-3",
				Type:       model.EventTypePullRequest,
				Action:     "closed",
				Repository: "test/repo",
				Sender:     "testuser",
				ReceivedAt: time.Now(),
				RawPayload: []byte(`{"action":"closed"}`),
			},
			wantErr: false, // Should not error, just log warning
		},
		{
			name: "Process unknown event type",
			event: &model.WebhookEvent{
				ID:         "test-delivery-4",
				Type:       model.EventTypeUnknown,
				Action:     "unknown",
				Repository: "test/repo",
				Sender:     "testuser",
				ReceivedAt: time.Now(),
				RawPayload: []byte(`{}`),
			},
			wantErr: false, // Should not error, just log warning
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := usecase.NewWebhook()
			ctx := context.Background()

			err := uc.ProcessEvent(ctx, tt.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("ProcessEvent() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
