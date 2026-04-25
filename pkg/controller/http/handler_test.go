package http_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/model/config"
	server "github.com/m-mizutani/shepherd/pkg/controller/http"
	"github.com/m-mizutani/shepherd/pkg/repository/memory"
	"github.com/m-mizutani/shepherd/pkg/usecase"
)

func setupTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	repo := memory.New()
	t.Cleanup(func() { _ = repo.Close() })

	registry := model.NewWorkspaceRegistry()
	registry.Register(&model.WorkspaceEntry{
		Workspace: model.Workspace{ID: "support", Name: "Support Team"},
		FieldSchema: &config.FieldSchema{
			Statuses: []config.StatusDef{
				{ID: "open", Name: "Open", Color: "#22c55e", Order: 0},
				{ID: "closed", Name: "Closed", Color: "#6b7280", Order: 1},
			},
			TicketConfig: config.TicketConfig{
				DefaultStatusID: "open",
				ClosedStatusIDs: []string{"closed"},
			},
			Fields: []config.FieldDefinition{
				{
					ID:   "priority",
					Name: "Priority",
					Type: "select",
					Options: []config.FieldOption{
						{ID: "high", Name: "High"},
						{ID: "low", Name: "Low"},
					},
				},
			},
			Labels: config.EntityLabels{
				Ticket:      "Ticket",
				Title:       "Title",
				Description: "Description",
			},
		},
		SlackChannelID: "C123",
	})

	authUC := usecase.NewNoAuthnUseCase("U_TEST", "test@example.com", "Test User")
	srv := server.New(registry, repo, authUC)
	return httptest.NewServer(srv)
}

func TestHealth(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/health")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]string
	json.NewDecoder(resp.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("expected status 'ok', got %q", body["status"])
	}
}

func TestListWorkspaces(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/ws")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		Workspaces []struct {
			Id   string `json:"id"`
			Name string `json:"name"`
		} `json:"workspaces"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if len(body.Workspaces) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(body.Workspaces))
	}
	if body.Workspaces[0].Id != "support" {
		t.Errorf("expected workspace 'support', got %q", body.Workspaces[0].Id)
	}
}

func TestGetWorkspace(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/ws/support")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestGetWorkspace_NotFound(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/ws/nonexistent")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetWorkspaceConfig(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/v1/ws/support/config")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var body struct {
		Statuses []struct {
			Id string `json:"id"`
		} `json:"statuses"`
		TicketConfig struct {
			DefaultStatusId string `json:"defaultStatusId"`
		} `json:"ticketConfig"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if len(body.Statuses) != 2 {
		t.Errorf("expected 2 statuses, got %d", len(body.Statuses))
	}
	if body.TicketConfig.DefaultStatusId != "open" {
		t.Errorf("expected default status 'open', got %q", body.TicketConfig.DefaultStatusId)
	}
}

func createTicketViaAPI(t *testing.T, ts *httptest.Server, wsID, title string) map[string]any {
	t.Helper()

	body := `{"title":"` + title + `"}`
	resp, err := http.Post(ts.URL+"/api/v1/ws/"+wsID+"/tickets", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, string(b))
	}

	var result map[string]any
	json.NewDecoder(resp.Body).Decode(&result)
	return result
}

func TestTicketCRUD(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	// Create
	ticket := createTicketViaAPI(t, ts, "support", "Test Ticket")
	ticketID, ok := ticket["id"].(string)
	if !ok || ticketID == "" {
		t.Fatal("expected ticket ID in response")
	}
	if ticket["statusId"] != "open" {
		t.Errorf("expected default status 'open', got %v", ticket["statusId"])
	}

	// Get
	resp, err := http.Get(ts.URL + "/api/v1/ws/support/tickets/" + ticketID)
	if err != nil {
		t.Fatalf("Get request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	// List
	resp, err = http.Get(ts.URL + "/api/v1/ws/support/tickets")
	if err != nil {
		t.Fatalf("List request failed: %v", err)
	}
	defer resp.Body.Close()
	var listBody struct {
		Tickets []struct {
			Id string `json:"id"`
		} `json:"tickets"`
	}
	json.NewDecoder(resp.Body).Decode(&listBody)
	if len(listBody.Tickets) != 1 {
		t.Errorf("expected 1 ticket, got %d", len(listBody.Tickets))
	}

	// Update
	updateBody := `{"title":"Updated Title","statusId":"closed"}`
	req, _ := http.NewRequest(http.MethodPatch, ts.URL+"/api/v1/ws/support/tickets/"+ticketID, strings.NewReader(updateBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Update request failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 200, got %d: %s", resp.StatusCode, string(b))
	}

	// Delete
	req, _ = http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/ws/support/tickets/"+ticketID, nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("Delete request failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204, got %d", resp.StatusCode)
	}
}

func TestTicketList_FilterByClosed(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	createTicketViaAPI(t, ts, "support", "Open Ticket")

	// Create a ticket then close it
	closed := createTicketViaAPI(t, ts, "support", "Closed Ticket")
	closedID := closed["id"].(string)
	updateBody := `{"statusId":"closed"}`
	req, _ := http.NewRequest(http.MethodPatch, ts.URL+"/api/v1/ws/support/tickets/"+closedID, strings.NewReader(updateBody))
	req.Header.Set("Content-Type", "application/json")
	http.DefaultClient.Do(req)

	// Filter open only
	resp, err := http.Get(ts.URL + "/api/v1/ws/support/tickets?isClosed=false")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	var body struct {
		Tickets []struct {
			Title string `json:"title"`
		} `json:"tickets"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	if len(body.Tickets) != 1 {
		t.Errorf("expected 1 open ticket, got %d", len(body.Tickets))
	}
}

func TestAuthMe_NoAuthn(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/auth/me")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAuthLogin_NoAuthn(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp, err := client.Get(ts.URL + "/api/auth/login")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusTemporaryRedirect && resp.StatusCode != http.StatusFound {
		t.Errorf("expected 302 or 307, got %d", resp.StatusCode)
	}
	loc := resp.Header.Get("Location")
	if loc != "/" {
		t.Errorf("expected redirect to '/', got %q", loc)
	}
}
