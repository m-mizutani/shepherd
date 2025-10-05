package http_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/google/go-github/v75/github"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/mock"
	"github.com/m-mizutani/gt"
	controller "github.com/m-mizutani/shepherd/pkg/controller/http"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/usecase"
)

//go:embed testdata/pr_opened_event.json
var prOpenedEventJSON []byte

// generateSignature generates HMAC-SHA256 signature for testing
func generateSignature(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestWebhookHandler_SignatureVerification(t *testing.T) {
	secret := "test-secret"
	uc := usecase.NewWebhook()
	handler := controller.NewWebhookHandler(secret, uc, nil)

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
	handler := controller.NewWebhookHandler(secret, uc, nil)

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
		nil, // pkgDetectorUC not needed for integration test
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

func TestWebhookHandler_PROpenedWithPackageDetection(t *testing.T) {
	secret := "test-secret"

	// Mock detection result
	detectionResult := model.PackageUpdateDetection{
		IsPackageUpdate: true,
		Language:        "Go",
		Packages: []model.PackageUpdate{
			{
				Name:        "github.com/m-mizutani/ctxlog",
				FromVersion: "v0.0.7",
				ToVersion:   "v0.0.8",
			},
		},
	}
	jsonData, _ := json.Marshal(detectionResult)

	// WaitGroup to synchronize async processing
	var wg sync.WaitGroup
	wg.Add(1)

	// Mock LLM client using gollem/mock - capture input for verification
	var capturedLLMInput string
	mockLLM := &mock.LLMClientMock{
		NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
			return &mock.SessionMock{
				GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
					// Capture the input text for verification
					for _, in := range input {
						if textInput, ok := in.(gollem.Text); ok {
							capturedLLMInput = string(textInput)
						}
					}
					return &gollem.Response{
						Texts: []string{string(jsonData)},
					}, nil
				},
			}, nil
		},
	}

	// Mock GitHub client - capture comment body for verification
	var capturedCommentBody string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture comment body from request
		var commentReq struct {
			Body string `json:"body"`
		}
		_ = json.NewDecoder(r.Body).Decode(&commentReq)
		capturedCommentBody = commentReq.Body

		// Mock GitHub API response for creating comment
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":   1,
			"body": commentReq.Body,
		})

		// Signal completion of async processing
		wg.Done()
	}))
	defer ts.Close()

	mockGitHub, _ := github.NewClient(nil).WithEnterpriseURLs(ts.URL, ts.URL)

	// Create real package detector usecase with mocks
	pkgDetectorUC, err := usecase.NewPackageDetector(mockLLM, mockGitHub)
	gt.NoError(t, err)

	uc := usecase.NewWebhook()
	handler := controller.NewWebhookHandler(secret, uc, pkgDetectorUC)

	// Use embedded real PR opened event JSON
	payload := prOpenedEventJSON
	signature := generateSignature(secret, payload)

	req := httptest.NewRequest(http.MethodPost, "/hooks/github/app", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", "pull_request")
	req.Header.Set("X-GitHub-Delivery", "test-pr-opened-delivery")
	req.Header.Set("X-Hub-Signature-256", signature)

	w := httptest.NewRecorder()
	handler.Handle(w, req)

	// Verify HTTP response
	gt.Equal(t, w.Code, http.StatusOK)

	// Wait for async processing to complete
	wg.Wait()

	// Verify LLM was called
	gt.V(t, len(mockLLM.NewSessionCalls())).NotEqual(0)

	// Verify LLM received expected input (PR title and body)
	gt.V(t, capturedLLMInput).NotEqual("")
	gt.V(t, strings.Contains(capturedLLMInput, "feat(async): implement asynchronous processing for webhook events")).Equal(true)

	// Verify GitHub comment was posted with expected content
	gt.V(t, capturedCommentBody).NotEqual("")
	gt.V(t, strings.Contains(capturedCommentBody, "## 📦 Package Update Detection")).Equal(true)
	gt.V(t, strings.Contains(capturedCommentBody, "**Language**: Go")).Equal(true)
	gt.V(t, strings.Contains(capturedCommentBody, "github.com/m-mizutani/ctxlog")).Equal(true)
	gt.V(t, strings.Contains(capturedCommentBody, "v0.0.7")).Equal(true)
	gt.V(t, strings.Contains(capturedCommentBody, "v0.0.8")).Equal(true)
}
