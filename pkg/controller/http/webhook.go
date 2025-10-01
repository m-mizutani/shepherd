package http

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/google/go-github/v75/github"
	"github.com/m-mizutani/ctxlog"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/domain/interfaces"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
)

// WebhookHandler handles GitHub webhooks
type WebhookHandler struct {
	secret    string
	webhookUC interfaces.WebhookUseCase
}

// NewWebhookHandler creates a new WebhookHandler
func NewWebhookHandler(secret string, webhookUC interfaces.WebhookUseCase) *WebhookHandler {
	return &WebhookHandler{
		secret:    secret,
		webhookUC: webhookUC,
	}
}

// Handle processes webhook requests
func (h *WebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := ctxlog.From(ctx)

	// Read payload
	body, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Error("Failed to read request body", "error", err)
		writeError(w, goerr.Wrap(err, "failed to read request body"), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Verify signature
	signature := r.Header.Get("X-Hub-Signature-256")
	if !h.verifySignature(body, signature) {
		logger.Warn("Invalid webhook signature")
		writeError(w, goerr.New("invalid signature"), http.StatusUnauthorized)
		return
	}

	// Parse event using GitHub SDK
	eventType := r.Header.Get("X-GitHub-Event")
	payload, err := github.ParseWebHook(eventType, body)
	if err != nil {
		logger.Error("Failed to parse webhook payload", "error", err)
		writeError(w, goerr.Wrap(err, "invalid JSON payload"), http.StatusBadRequest)
		return
	}

	// Create webhook event
	event := &model.WebhookEvent{
		ID:         r.Header.Get("X-GitHub-Delivery"),
		Type:       model.WebhookEventType(eventType),
		ReceivedAt: time.Now(),
		RawPayload: body,
	}

	// Extract event-specific information using GitHub SDK types
	switch e := payload.(type) {
	case *github.PullRequestEvent:
		if e.Action != nil {
			event.Action = *e.Action
		}
		if e.Repo != nil && e.Repo.FullName != nil {
			event.Repository = *e.Repo.FullName
		}
		if e.Sender != nil && e.Sender.Login != nil {
			event.Sender = *e.Sender.Login
		}
	case *github.ReleaseEvent:
		if e.Action != nil {
			event.Action = *e.Action
		}
		if e.Repo != nil && e.Repo.FullName != nil {
			event.Repository = *e.Repo.FullName
		}
		if e.Sender != nil && e.Sender.Login != nil {
			event.Sender = *e.Sender.Login
		}
	default:
		event.Type = model.EventTypeUnknown
	}

	// Process event via UseCase
	if err := h.webhookUC.ProcessEvent(ctx, event); err != nil {
		logger.Error("Failed to process webhook event", "error", err)
		writeError(w, err, http.StatusInternalServerError)
		return
	}

	// Success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status": "success",
	}); err != nil {
		logger.Error("Failed to encode success response", "error", err)
	}
}

// verifySignature verifies the webhook signature
func (h *WebhookHandler) verifySignature(payload []byte, signature string) bool {
	if signature == "" {
		return false
	}

	// Remove "sha256=" prefix if present
	signature = strings.TrimPrefix(signature, "sha256=")

	// Calculate HMAC-SHA256
	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write(payload)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expectedMAC))
}
