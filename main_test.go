package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	projectctx "seek/context"
	historypkg "seek/history"
)

func TestRunWithArgsHistoryFlagOutputsTable(t *testing.T) {
	prepareHistoryConfig(t)
	store := mustCLIHistoryStore(t)
	if _, err := store.Save(&historypkg.SearchRecord{
		Query:        "tcp handshake",
		Response:     "SYN, SYN-ACK, ACK",
		ProjectDir:   "/workspace/project",
		ProjectStack: "go/chi",
		LLMBackend:   "fake/model",
		OutputFormat: "concise",
		TotalMs:      250,
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close store: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := runWithArgs([]string{"--history", "tcp"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{"ID", "When", "Stack", "Query", "tcp handshake"} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected output to contain %q, got %q", want, output)
		}
	}
}

func TestRunWithArgsRecentFlagOutputsMostRecentRows(t *testing.T) {
	prepareHistoryConfig(t)
	store := mustCLIHistoryStore(t)
	if _, err := store.Save(&historypkg.SearchRecord{Query: "older", Response: "older", ProjectDir: "/workspace/project", TotalMs: 100}); err != nil {
		t.Fatalf("Save older: %v", err)
	}
	if _, err := store.Save(&historypkg.SearchRecord{Query: "newer", Response: "newer", ProjectDir: "/workspace/project", TotalMs: 120}); err != nil {
		t.Fatalf("Save newer: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close store: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := runWithArgs([]string{"--recent", "1"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "newer") || strings.Contains(output, "older") {
		t.Fatalf("expected only the most recent row, got %q", output)
	}
}

func TestRunWithArgsStatsFlagOutputsAggregates(t *testing.T) {
	prepareHistoryConfig(t)
	store := mustCLIHistoryStore(t)
	for _, totalMs := range []int64{100, 200, 300} {
		if _, err := store.Save(&historypkg.SearchRecord{
			Query:      "stats",
			Response:   "stats",
			ProjectDir: "/workspace/project",
			TotalMs:    totalMs,
		}); err != nil {
			t.Fatalf("Save stats record: %v", err)
		}
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close store: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := runWithArgs([]string{"--stats"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{"Total searches: 3", "Unique projects: 1", "Avg latency: 200ms"} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected stats output to contain %q, got %q", want, output)
		}
	}
}

func TestRunWithArgsClearHistoryDeletesSavedEntries(t *testing.T) {
	prepareHistoryConfig(t)
	store := mustCLIHistoryStore(t)
	if _, err := store.Save(&historypkg.SearchRecord{
		Query:      "wipe me",
		Response:   "response",
		ProjectDir: "/workspace/project",
		TotalMs:    100,
	}); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close store: %v", err)
	}

	var stdout, stderr bytes.Buffer
	code := runWithArgs([]string{"--clear-history"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected exit code 0, got %d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "Cleared 1 history entry.") {
		t.Fatalf("expected clear-history output, got %q", stdout.String())
	}

	store = mustCLIHistoryStore(t)
	defer store.Close()
	records, err := store.Recent(10, "")
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(records) != 0 {
		t.Fatalf("expected empty history after clear, got %#v", records)
	}
}

func TestRunWithArgsRejectsUnknownBackendOverride(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := runWithArgs([]string{"--backend", "fake", "test"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected exit code 2, got %d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), `unknown backend "fake"`) {
		t.Fatalf("expected backend validation error, got %q", stderr.String())
	}
}

func TestResolveSessionProjectContextUsesSavedProjectDir(t *testing.T) {
	cwd := t.TempDir()
	projectDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte("module example.com/test\nrequire github.com/go-chi/chi/v5 v5.0.0\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	workingDir, ctx := resolveSessionProjectContext(cwd, &historypkg.SearchRecord{
		ProjectDir:   projectDir,
		ProjectStack: "go/chi",
	})
	if workingDir != filepath.Clean(projectDir) {
		t.Fatalf("expected working dir %q, got %q", projectDir, workingDir)
	}
	if ctx == nil || ctx.Language != "go" || ctx.Framework != "chi" {
		t.Fatalf("expected detected context from saved project dir, got %#v", ctx)
	}
}

func TestResolveSessionProjectContextFallsBackToSavedStack(t *testing.T) {
	cwd := t.TempDir()
	projectDir := filepath.Join(t.TempDir(), "missing-project")

	workingDir, ctx := resolveSessionProjectContext(cwd, &historypkg.SearchRecord{
		ProjectDir:   projectDir,
		ProjectStack: "rust/axum",
	})
	if workingDir != filepath.Clean(projectDir) {
		t.Fatalf("expected working dir %q, got %q", projectDir, workingDir)
	}
	if ctx == nil || ctx.Language != "rust" || ctx.Framework != "axum" || ctx.RootDir != filepath.Clean(projectDir) {
		t.Fatalf("expected fallback context from saved stack, got %#v", ctx)
	}
}

func TestResolveSessionProjectContextDefaultsToCurrentDir(t *testing.T) {
	cwd := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwd, "go.mod"), []byte("module example.com/local\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	workingDir, ctx := resolveSessionProjectContext(cwd, nil)
	if workingDir != filepath.Clean(cwd) {
		t.Fatalf("expected cwd %q, got %q", cwd, workingDir)
	}
	if ctx == nil || ctx.Language != projectctx.DetectContext(cwd).Language {
		t.Fatalf("expected cwd-derived context, got %#v", ctx)
	}
}

func prepareHistoryConfig(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	configDir := filepath.Join(home, ".config", "seek")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte("history_enabled = true\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func mustCLIHistoryStore(t *testing.T) *historypkg.HistoryStore {
	t.Helper()
	store, err := historypkg.NewHistoryStore(DefaultHistoryDBPath())
	if err != nil {
		t.Fatalf("NewHistoryStore: %v", err)
	}
	return store
}
