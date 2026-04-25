package config_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/m-mizutani/gt"
	"github.com/m-mizutani/shepherd/pkg/cli/config"
)

func writeToml(t *testing.T, dir, filename, content string) string {
	t.Helper()
	path := filepath.Join(dir, filename)
	gt.NoError(t, os.WriteFile(path, []byte(content), 0o644)).Required()
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

	configs := gt.R1(config.LoadWorkspaceConfigs([]string{path})).NoError(t)
	gt.A(t, configs).Length(1)
	gt.S(t, configs[0].ID).Equal("test-ws")
	gt.S(t, configs[0].Name).Equal("Test Workspace")
	gt.S(t, configs[0].SlackChannel).Equal("C0123456789")
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

	configs := gt.R1(config.LoadWorkspaceConfigs([]string{dir})).NoError(t)
	gt.A(t, configs).Length(2)
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
	gt.Error(t, err)
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
	gt.Error(t, err)
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
	gt.Error(t, err)
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
	gt.Error(t, err)
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
	gt.Error(t, err)
}

func TestLoadWorkspaceConfigs_InvalidTOML(t *testing.T) {
	dir := t.TempDir()
	path := writeToml(t, dir, "bad.toml", "this is not valid toml {{{}}")

	_, err := config.LoadWorkspaceConfigs([]string{path})
	gt.Error(t, err)
}

func TestLoadWorkspaceConfigs_NonexistentPath(t *testing.T) {
	_, err := config.LoadWorkspaceConfigs([]string{"/nonexistent/path"})
	gt.Error(t, err)
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

	configs := gt.R1(config.LoadWorkspaceConfigs([]string{path})).NoError(t)
	gt.S(t, configs[0].FieldSchema.Labels.Ticket).Equal("Ticket")
	gt.S(t, configs[0].FieldSchema.Labels.Title).Equal("Title")
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

	configs := gt.R1(config.LoadWorkspaceConfigs([]string{path})).NoError(t)
	gt.S(t, configs[0].FieldSchema.TicketConfig.DefaultStatusID).Equal("new")
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

	configs := gt.R1(config.LoadWorkspaceConfigs([]string{path})).NoError(t)
	gt.S(t, configs[0].Name).Equal("my-ws")
}

func TestBuildRegistry(t *testing.T) {
	dir := t.TempDir()
	writeToml(t, dir, "ws.toml", validTOML)

	configs := gt.R1(config.LoadWorkspaceConfigs([]string{dir})).NoError(t)

	ctx := context.Background()
	registry := gt.R1(config.BuildRegistry(ctx, configs, nil)).NoError(t)
	entry, ok := registry.Get("test-ws")
	gt.B(t, ok).True()
	gt.S(t, entry.Workspace.Name).Equal("Test Workspace")
	gt.S(t, entry.SlackChannelID).Equal("C0123456789")
}
