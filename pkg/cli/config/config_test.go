package config_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/m-mizutani/shepherd/pkg/cli/config"
)

func writeToml(t *testing.T, dir, filename, content string) string {
	t.Helper()
	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
	return path
}

const validTOML = `
[workspace]
id = "test-ws"
name = "Test Workspace"

[ticket]
default_status = "open"
closed_statuses = ["closed"]

[slack]
channel = "C0123456789"

[[statuses]]
id = "open"
name = "Open"
color = "#22c55e"

[[statuses]]
id = "closed"
name = "Closed"
color = "#6b7280"

[labels]
ticket = "Ticket"
title = "Title"
description = "Description"

[[fields]]
id = "priority"
name = "Priority"
type = "select"
required = true

  [[fields.options]]
  id = "high"
  name = "High"

  [[fields.options]]
  id = "low"
  name = "Low"
`

func TestLoadWorkspaceConfigs_SingleFile(t *testing.T) {
	dir := t.TempDir()
	path := writeToml(t, dir, "ws.toml", validTOML)

	configs, err := config.LoadWorkspaceConfigs([]string{path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(configs) != 1 {
		t.Fatalf("expected 1 config, got %d", len(configs))
	}
	if configs[0].ID != "test-ws" {
		t.Errorf("expected ID 'test-ws', got %q", configs[0].ID)
	}
	if configs[0].Name != "Test Workspace" {
		t.Errorf("expected Name 'Test Workspace', got %q", configs[0].Name)
	}
	if configs[0].SlackChannel != "C0123456789" {
		t.Errorf("expected SlackChannel 'C0123456789', got %q", configs[0].SlackChannel)
	}
}

func TestLoadWorkspaceConfigs_Directory(t *testing.T) {
	dir := t.TempDir()
	writeToml(t, dir, "a.toml", validTOML)

	ws2 := `
[workspace]
id = "ws-two"
[slack]
channel = "C9999"
[[statuses]]
id = "open"
name = "Open"
color = "#fff"
`
	writeToml(t, dir, "b.toml", ws2)

	configs, err := config.LoadWorkspaceConfigs([]string{dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(configs) != 2 {
		t.Fatalf("expected 2 configs, got %d", len(configs))
	}
}

func TestLoadWorkspaceConfigs_DuplicateWorkspaceID(t *testing.T) {
	dir := t.TempDir()
	writeToml(t, dir, "a.toml", validTOML)

	dup := `
[workspace]
id = "test-ws"
[slack]
channel = "C9999"
[[statuses]]
id = "open"
name = "Open"
color = "#fff"
`
	writeToml(t, dir, "b.toml", dup)

	_, err := config.LoadWorkspaceConfigs([]string{dir})
	if err == nil {
		t.Fatal("expected error for duplicate workspace ID")
	}
}

func TestLoadWorkspaceConfigs_DuplicateChannelID(t *testing.T) {
	dir := t.TempDir()
	writeToml(t, dir, "a.toml", validTOML)

	dup := `
[workspace]
id = "other-ws"
[slack]
channel = "C0123456789"
[[statuses]]
id = "open"
name = "Open"
color = "#fff"
`
	writeToml(t, dir, "b.toml", dup)

	_, err := config.LoadWorkspaceConfigs([]string{dir})
	if err == nil {
		t.Fatal("expected error for duplicate channel ID")
	}
}

func TestLoadWorkspaceConfigs_MissingWorkspaceID(t *testing.T) {
	dir := t.TempDir()
	noID := `
[workspace]
name = "No ID"
[slack]
channel = "C111"
[[statuses]]
id = "open"
name = "Open"
color = "#fff"
`
	path := writeToml(t, dir, "bad.toml", noID)

	_, err := config.LoadWorkspaceConfigs([]string{path})
	if err == nil {
		t.Fatal("expected error for missing workspace ID")
	}
}

func TestLoadWorkspaceConfigs_InvalidWorkspaceID(t *testing.T) {
	dir := t.TempDir()
	badID := `
[workspace]
id = "INVALID_ID"
[slack]
channel = "C111"
[[statuses]]
id = "open"
name = "Open"
color = "#fff"
`
	path := writeToml(t, dir, "bad.toml", badID)

	_, err := config.LoadWorkspaceConfigs([]string{path})
	if err == nil {
		t.Fatal("expected error for invalid workspace ID format")
	}
}

func TestLoadWorkspaceConfigs_MissingChannelID(t *testing.T) {
	dir := t.TempDir()
	noCh := `
[workspace]
id = "test-ws"
[[statuses]]
id = "open"
name = "Open"
color = "#fff"
`
	path := writeToml(t, dir, "bad.toml", noCh)

	_, err := config.LoadWorkspaceConfigs([]string{path})
	if err == nil {
		t.Fatal("expected error for missing channel ID")
	}
}

func TestLoadWorkspaceConfigs_InvalidTOML(t *testing.T) {
	dir := t.TempDir()
	path := writeToml(t, dir, "bad.toml", "this is not valid toml {{{}}")

	_, err := config.LoadWorkspaceConfigs([]string{path})
	if err == nil {
		t.Fatal("expected error for invalid TOML syntax")
	}
}

func TestLoadWorkspaceConfigs_NonexistentPath(t *testing.T) {
	_, err := config.LoadWorkspaceConfigs([]string{"/nonexistent/path"})
	if err == nil {
		t.Fatal("expected error for nonexistent path")
	}
}

func TestLoadWorkspaceConfigs_DefaultLabels(t *testing.T) {
	dir := t.TempDir()
	noLabels := `
[workspace]
id = "test-ws"
[slack]
channel = "C111"
[[statuses]]
id = "open"
name = "Open"
color = "#fff"
`
	path := writeToml(t, dir, "ws.toml", noLabels)

	configs, err := config.LoadWorkspaceConfigs([]string{path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	labels := configs[0].FieldSchema.Labels
	if labels.Ticket != "Ticket" {
		t.Errorf("expected default label 'Ticket', got %q", labels.Ticket)
	}
	if labels.Title != "Title" {
		t.Errorf("expected default label 'Title', got %q", labels.Title)
	}
}

func TestLoadWorkspaceConfigs_DefaultStatusFromFirst(t *testing.T) {
	dir := t.TempDir()
	noDefault := `
[workspace]
id = "test-ws"
[slack]
channel = "C111"
[[statuses]]
id = "new"
name = "New"
color = "#fff"
[[statuses]]
id = "done"
name = "Done"
color = "#000"
`
	path := writeToml(t, dir, "ws.toml", noDefault)

	configs, err := config.LoadWorkspaceConfigs([]string{path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if configs[0].FieldSchema.TicketConfig.DefaultStatusID != "new" {
		t.Errorf("expected default status 'new', got %q", configs[0].FieldSchema.TicketConfig.DefaultStatusID)
	}
}

func TestLoadWorkspaceConfigs_NameFallsBackToID(t *testing.T) {
	dir := t.TempDir()
	noName := `
[workspace]
id = "my-ws"
[slack]
channel = "C111"
[[statuses]]
id = "open"
name = "Open"
color = "#fff"
`
	path := writeToml(t, dir, "ws.toml", noName)

	configs, err := config.LoadWorkspaceConfigs([]string{path})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if configs[0].Name != "my-ws" {
		t.Errorf("expected name to fall back to ID 'my-ws', got %q", configs[0].Name)
	}
}

func TestBuildRegistry(t *testing.T) {
	dir := t.TempDir()
	writeToml(t, dir, "ws.toml", validTOML)

	configs, err := config.LoadWorkspaceConfigs([]string{dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	registry, err := config.BuildRegistry(ctx, configs, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	entry, ok := registry.Get("test-ws")
	if !ok {
		t.Fatal("expected workspace 'test-ws' in registry")
	}
	if entry.Workspace.Name != "Test Workspace" {
		t.Errorf("expected name 'Test Workspace', got %q", entry.Workspace.Name)
	}
	if entry.SlackChannelID != "C0123456789" {
		t.Errorf("expected channel 'C0123456789', got %q", entry.SlackChannelID)
	}
}
