package agentstore_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/trace"
	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/adapter/agentstore"
)

func TestFileBackend_HistoryRoundTrip(t *testing.T) {
	dir := t.TempDir()
	be := gt.R1(agentstore.NewFileBackend(dir)).NoError(t)
	repo := agentstore.NewHistoryRepository(be)

	ctx := context.Background()
	const sessionID = "ws-1/ticket-42"

	// Missing → (nil, nil)
	got, err := repo.Load(ctx, sessionID)
	gt.NoError(t, err)
	gt.Nil(t, got)

	in := &gollem.History{
		LLType:  gollem.LLMTypeOpenAI,
		Version: gollem.HistoryVersion,
	}
	gt.NoError(t, repo.Save(ctx, sessionID, in))

	// Stored at history/v1/ws-1/ticket-42.json
	path := filepath.Join(dir, "history", "v1", "ws-1", "ticket-42.json")
	_ = gt.R1(os.Stat(path)).NoError(t)

	loaded, err := repo.Load(ctx, sessionID)
	gt.NoError(t, err)
	gt.NotNil(t, loaded)
	gt.V(t, loaded.Version).Equal(gollem.HistoryVersion)
}

func TestFileBackend_HistoryVersionMismatch_StartsFresh(t *testing.T) {
	dir := t.TempDir()
	be := gt.R1(agentstore.NewFileBackend(dir)).NoError(t)
	repo := agentstore.NewHistoryRepository(be)

	// Pre-write a payload with an unknown version.
	path := filepath.Join(dir, "history", "v1", "ws-x", "ticket-y.json")
	gt.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	gt.NoError(t, os.WriteFile(path, []byte(`{"type":"OpenAI","version":1,"messages":[]}`), 0o600))

	got, err := repo.Load(context.Background(), "ws-x/ticket-y")
	gt.NoError(t, err)
	gt.Nil(t, got)
}

func TestFileBackend_TraceSave(t *testing.T) {
	dir := t.TempDir()
	be := gt.R1(agentstore.NewFileBackend(dir)).NoError(t)
	repo := agentstore.NewTraceRepository(be)

	tr := &trace.Trace{
		TraceID:   "tr-001",
		StartedAt: time.Unix(0, 0).UTC(),
		EndedAt:   time.Unix(1, 0).UTC(),
		Metadata: trace.TraceMetadata{
			Labels: map[string]string{"seq": "1", "ticket_id": "T-9"},
		},
	}
	gt.NoError(t, repo.Save(context.Background(), tr))

	path := filepath.Join(dir, "trace", "v1", "tr-001.json")
	data := gt.R1(os.ReadFile(path)).NoError(t)

	var decoded trace.Trace
	gt.NoError(t, json.Unmarshal(data, &decoded))
	gt.S(t, decoded.TraceID).Equal("tr-001")
	gt.V(t, decoded.Metadata.Labels["seq"]).Equal("1")
}

func TestFileBackend_GetMissingReturnsNilNil(t *testing.T) {
	dir := t.TempDir()
	be := gt.R1(agentstore.NewFileBackend(dir)).NoError(t)

	rc, err := be.Get(context.Background(), "history/v1/none.json")
	gt.NoError(t, err)
	gt.Nil(t, rc)
}
