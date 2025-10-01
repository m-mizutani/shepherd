package http

import (
	"encoding/json"
	"net/http"

	"github.com/m-mizutani/ctxlog"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/types"
)

// handleHealth handles health check requests
func handleHealth(w http.ResponseWriter, r *http.Request) {
	status := &model.HealthStatus{
		Status:  "healthy",
		Service: "shepherd",
		Version: types.Version,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(status); err != nil {
		ctxlog.From(r.Context()).Error("Failed to encode health response", "error", err)
	}
}
