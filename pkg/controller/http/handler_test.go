package http_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/m-mizutani/gt"
	server "github.com/m-mizutani/shepherd/pkg/controller/http"
	"github.com/m-mizutani/shepherd/pkg/domain/model"
	"github.com/m-mizutani/shepherd/pkg/domain/model/config"
	"github.com/m-mizutani/shepherd/pkg/repository/memory"
	"github.com/m-mizutani/shepherd/pkg/usecase"
	"github.com/m-mizutani/shepherd/pkg/utils/safe"
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

func doGet(t *testing.T, url string) *http.Response {
	t.Helper()
	resp := gt.R1(http.Get(url)).NoError(t)
	t.Cleanup(func() { safe.Close(context.Background(), resp.Body) })
	return resp
}

func decodeJSON[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	var v T
	gt.NoError(t, json.NewDecoder(resp.Body).Decode(&v)).Required()
	return v
}

func createTicketViaAPI(t *testing.T, ts *httptest.Server, wsID, title string) map[string]any {
	t.Helper()

	body := `{"title":"` + title + `"}`
	resp := gt.R1(http.Post(ts.URL+"/api/v1/ws/"+wsID+"/tickets", "application/json", strings.NewReader(body))).NoError(t)
	t.Cleanup(func() { safe.Close(context.Background(), resp.Body) })

	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, string(b))
	}

	return decodeJSON[map[string]any](t, resp)
}

func TestHealth(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp := doGet(t, ts.URL+"/api/v1/health")
	gt.N(t, resp.StatusCode).Equal(http.StatusOK)

	body := decodeJSON[map[string]string](t, resp)
	gt.S(t, body["status"]).Equal("ok")
}

func TestListWorkspaces(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp := doGet(t, ts.URL+"/api/v1/ws")
	gt.N(t, resp.StatusCode).Equal(http.StatusOK)

	var body struct {
		Workspaces []struct {
			Id   string `json:"id"`
			Name string `json:"name"`
		} `json:"workspaces"`
	}
	body = decodeJSON[struct {
		Workspaces []struct {
			Id   string `json:"id"`
			Name string `json:"name"`
		} `json:"workspaces"`
	}](t, resp)
	gt.A(t, body.Workspaces).Length(1)
	gt.S(t, body.Workspaces[0].Id).Equal("support")
}

func TestGetWorkspace(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp := doGet(t, ts.URL+"/api/v1/ws/support")
	gt.N(t, resp.StatusCode).Equal(http.StatusOK)
}

func TestGetWorkspace_NotFound(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp := doGet(t, ts.URL+"/api/v1/ws/nonexistent")
	gt.N(t, resp.StatusCode).Equal(http.StatusNotFound)
}

func TestGetWorkspaceConfig(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp := doGet(t, ts.URL+"/api/v1/ws/support/config")
	gt.N(t, resp.StatusCode).Equal(http.StatusOK)

	type configResp struct {
		Statuses []struct {
			Id string `json:"id"`
		} `json:"statuses"`
		TicketConfig struct {
			DefaultStatusId string `json:"defaultStatusId"`
		} `json:"ticketConfig"`
	}
	body := decodeJSON[configResp](t, resp)
	gt.A(t, body.Statuses).Length(2)
	gt.S(t, body.TicketConfig.DefaultStatusId).Equal("open")
}

func TestTicketCRUD(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	// Create
	ticket := createTicketViaAPI(t, ts, "support", "Test Ticket")
	ticketID, ok := ticket["id"].(string)
	gt.B(t, ok).True()
	gt.S(t, ticketID).NotEqual("")
	gt.V(t, ticket["statusId"]).Equal(any("open"))

	// Get
	resp := doGet(t, ts.URL+"/api/v1/ws/support/tickets/"+ticketID)
	gt.N(t, resp.StatusCode).Equal(http.StatusOK)

	// List
	resp = doGet(t, ts.URL+"/api/v1/ws/support/tickets")
	type listResp struct {
		Tickets []struct {
			Id string `json:"id"`
		} `json:"tickets"`
	}
	listBody := decodeJSON[listResp](t, resp)
	gt.A(t, listBody.Tickets).Length(1)

	// Update
	updateBody := `{"title":"Updated Title","statusId":"closed"}`
	req := gt.R1(http.NewRequest(http.MethodPatch, ts.URL+"/api/v1/ws/support/tickets/"+ticketID, strings.NewReader(updateBody))).NoError(t)
	req.Header.Set("Content-Type", "application/json")
	resp = gt.R1(http.DefaultClient.Do(req)).NoError(t)
	t.Cleanup(func() { safe.Close(context.Background(), resp.Body) })
	gt.N(t, resp.StatusCode).Equal(http.StatusOK)

	// Delete
	req = gt.R1(http.NewRequest(http.MethodDelete, ts.URL+"/api/v1/ws/support/tickets/"+ticketID, nil)).NoError(t)
	resp = gt.R1(http.DefaultClient.Do(req)).NoError(t)
	safe.Close(context.Background(), resp.Body)
	gt.N(t, resp.StatusCode).Equal(http.StatusNoContent)
}

func TestTicketList_FilterByClosed(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	createTicketViaAPI(t, ts, "support", "Open Ticket")

	closed := createTicketViaAPI(t, ts, "support", "Closed Ticket")
	closedID := closed["id"].(string)
	updateBody := `{"statusId":"closed"}`
	req := gt.R1(http.NewRequest(http.MethodPatch, ts.URL+"/api/v1/ws/support/tickets/"+closedID, strings.NewReader(updateBody))).NoError(t)
	req.Header.Set("Content-Type", "application/json")
	gt.R1(http.DefaultClient.Do(req)).NoError(t)

	resp := doGet(t, ts.URL+"/api/v1/ws/support/tickets?isClosed=false")
	type filterResp struct {
		Tickets []struct {
			Title string `json:"title"`
		} `json:"tickets"`
	}
	body := decodeJSON[filterResp](t, resp)
	gt.A(t, body.Tickets).Length(1)
}

func TestAuthMe_NoAuthn(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	resp := doGet(t, ts.URL+"/api/auth/me")
	gt.N(t, resp.StatusCode).Equal(http.StatusOK)
}

func TestAuthLogin_NoAuthn(t *testing.T) {
	ts := setupTestServer(t)
	defer ts.Close()

	client := &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	resp := gt.R1(client.Get(ts.URL + "/api/auth/login")).NoError(t)
	t.Cleanup(func() { safe.Close(context.Background(), resp.Body) })

	gt.B(t, resp.StatusCode == http.StatusTemporaryRedirect || resp.StatusCode == http.StatusFound).True()
	gt.S(t, resp.Header.Get("Location")).Equal("/")
}
