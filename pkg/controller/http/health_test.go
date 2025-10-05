package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/m-mizutani/shepherd/pkg/domain/model"
	controller "github.com/m-mizutani/shepherd/pkg/controller/http"
	"github.com/m-mizutani/shepherd/pkg/usecase"
)

func TestHealthEndpoint(t *testing.T) {
	ctx := context.Background()
	uc := usecase.NewWebhook()

	server, err := controller.NewServer(
		ctx,
		uc,
		nil, // pkgDetectorUC not needed for health check test
		controller.WithAddr("localhost:0"),
		controller.WithWebhookSecret("test-secret"),
	)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	server.Handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status code = %v, want %v", w.Code, http.StatusOK)
	}

	var status model.HealthStatus
	if err := json.NewDecoder(w.Body).Decode(&status); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if status.Status != "healthy" {
		t.Errorf("Status = %v, want healthy", status.Status)
	}

	if status.Service != "shepherd" {
		t.Errorf("Service = %v, want shepherd", status.Service)
	}

	if status.Version == "" {
		t.Error("Version should not be empty")
	}
}
