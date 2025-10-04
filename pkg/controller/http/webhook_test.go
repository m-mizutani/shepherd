package http_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	controller "github.com/m-mizutani/shepherd/pkg/controller/http"
	"github.com/m-mizutani/shepherd/pkg/usecase"
)

// generateSignature generates HMAC-SHA256 signature for testing
func generateSignature(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestWebhookHandler_SignatureVerification(t *testing.T) {
	secret := "test-secret"
	uc := usecase.NewWebhook()
	handler := controller.NewWebhookHandler(secret, uc)

	tests := []struct {
		name           string
		payload        string
		signature      string
		wantStatusCode int
	}{
		{
			name:           "Valid signature",
			payload:        `{"action":"opened","pull_request":{"id":1},"repository":{"full_name":"test/repo"},"sender":{"login":"testuser"}}`,
			signature:      "", // Will be generated
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "Invalid signature",
			payload:        `{"action":"opened"}`,
			signature:      "sha256=invalid",
			wantStatusCode: http.StatusUnauthorized,
		},
		{
			name:           "Missing signature",
			payload:        `{"action":"opened"}`,
			signature:      "",
			wantStatusCode: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := []byte(tt.payload)
			signature := tt.signature
			if signature == "" && tt.wantStatusCode == http.StatusOK {
				signature = generateSignature(secret, payload)
			}

			req := httptest.NewRequest(http.MethodPost, "/hooks/github/app", bytes.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-GitHub-Event", "pull_request")
			req.Header.Set("X-GitHub-Delivery", "test-delivery")
			req.Header.Set("X-Hub-Signature-256", signature)

			w := httptest.NewRecorder()
			handler.Handle(w, req)

			if w.Code != tt.wantStatusCode {
				t.Errorf("Handle() status = %v, want %v", w.Code, tt.wantStatusCode)
			}
		})
	}
}

func TestWebhookHandler_EventParsing(t *testing.T) {
	secret := "test-secret"
	uc := usecase.NewWebhook()
	handler := controller.NewWebhookHandler(secret, uc)

	tests := []struct {
		name           string
		eventType      string
		payload        map[string]interface{}
		wantStatusCode int
	}{
		{
			name:      "Pull Request opened event",
			eventType: "pull_request",
			payload: map[string]interface{}{
				"action": "opened",
				"pull_request": map[string]interface{}{
					"id": 1,
				},
				"repository": map[string]interface{}{
					"full_name": "test/repo",
				},
				"sender": map[string]interface{}{
					"login": "testuser",
				},
			},
			wantStatusCode: http.StatusOK,
		},
		{
			name:      "Release released event",
			eventType: "release",
			payload: map[string]interface{}{
				"action": "released",
				"release": map[string]interface{}{
					"id": 1,
				},
				"repository": map[string]interface{}{
					"full_name": "test/repo",
				},
				"sender": map[string]interface{}{
					"login": "testuser",
				},
			},
			wantStatusCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payloadBytes, _ := json.Marshal(tt.payload)
			signature := generateSignature(secret, payloadBytes)

			req := httptest.NewRequest(http.MethodPost, "/hooks/github/app", bytes.NewReader(payloadBytes))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-GitHub-Event", tt.eventType)
			req.Header.Set("X-GitHub-Delivery", "test-delivery")
			req.Header.Set("X-Hub-Signature-256", signature)

			w := httptest.NewRecorder()
			handler.Handle(w, req)

			if w.Code != tt.wantStatusCode {
				t.Errorf("Handle() status = %v, want %v, body = %s", w.Code, tt.wantStatusCode, w.Body.String())
			}

			if tt.wantStatusCode == http.StatusOK {
				var response map[string]string
				if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
					t.Errorf("Failed to decode response: %v", err)
				}
				if response["status"] != "success" {
					t.Errorf("Response status = %v, want success", response["status"])
				}
			}
		})
	}
}

func TestWebhookHandler_Integration(t *testing.T) {
	ctx := context.Background()
	secret := "integration-test-secret"
	uc := usecase.NewWebhook()

	server, err := controller.NewServer(
		ctx,
		uc,
		controller.WithAddr("localhost:0"),
		controller.WithWebhookSecret(secret),
	)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	ts := httptest.NewServer(server.Handler)
	defer ts.Close()

	payload := map[string]interface{}{
		"action": "opened",
		"pull_request": map[string]interface{}{
			"id": 1,
		},
		"repository": map[string]interface{}{
			"full_name": "test/repo",
		},
		"sender": map[string]interface{}{
			"login": "testuser",
		},
	}

	payloadBytes, _ := json.Marshal(payload)
	signature := generateSignature(secret, payloadBytes)

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/hooks/github/app", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "pull_request")
	req.Header.Set("X-GitHub-Delivery", "integration-test")
	req.Header.Set("X-Hub-Signature-256", signature)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Failed to send request: %v", err)
	}
	defer func() {
		_ = resp.Body.Close() // Error ignored in test
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Status code = %v, want %v", resp.StatusCode, http.StatusOK)
	}
}
