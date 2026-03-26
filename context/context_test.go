package context

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectContextGoProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), `
module example.com/myapp

go 1.23.0

require (
    github.com/go-chi/chi/v5 v5.0.0
    github.com/jmoiron/sqlx v1.3.0
)
`)

	ctx := DetectContext(dir)
	if ctx == nil {
		t.Fatal("expected go project context")
	}
	if ctx.Language != "go" || ctx.Framework != "chi" {
		t.Fatalf("unexpected context: %#v", ctx)
	}
	if ctx.GoVersion != "1.23.0" {
		t.Fatalf("expected go version, got %q", ctx.GoVersion)
	}
	if !containsAll(ctx.Dependencies, "chi", "sqlx") {
		t.Fatalf("expected chi/sqlx deps, got %#v", ctx.Dependencies)
	}
}

func TestDetectContextNodeTypeScriptProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{
  "dependencies": {
    "next": "14.0.0",
    "react": "19.0.0"
  },
  "devDependencies": {
    "typescript": "5.0.0"
  }
}`)

	ctx := DetectContext(dir)
	if ctx == nil {
		t.Fatal("expected node context")
	}
	if ctx.Language != "typescript" || ctx.Framework != "nextjs" {
		t.Fatalf("unexpected context: %#v", ctx)
	}
	if !containsAll(ctx.Dependencies, "next") {
		t.Fatalf("expected next dependency, got %#v", ctx.Dependencies)
	}
}

func TestDetectContextRustProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Cargo.toml"), `
[package]
name = "demo"

[dependencies]
axum = "0.7"
tokio = { version = "1", features = ["full"] }
`)

	ctx := DetectContext(dir)
	if ctx == nil {
		t.Fatal("expected rust context")
	}
	if ctx.Language != "rust" || ctx.Framework != "axum" {
		t.Fatalf("unexpected context: %#v", ctx)
	}
	if !containsAll(ctx.Dependencies, "axum") {
		t.Fatalf("expected axum dependency, got %#v", ctx.Dependencies)
	}
}

func TestDetectContextPythonFastAPIProject(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "requirements.txt"), "fastapi==0.100.0\nuvicorn==0.23.0\nsqlalchemy==2.0.0\n")

	ctx := DetectContext(dir)
	if ctx == nil {
		t.Fatal("expected python context")
	}
	if ctx.Language != "python" || ctx.Framework != "fastapi" {
		t.Fatalf("unexpected context: %#v", ctx)
	}
	if !containsAll(ctx.Dependencies, "fastapi", "uvicorn", "sqlalchemy") {
		t.Fatalf("expected requirements deps, got %#v", ctx.Dependencies)
	}
}

func TestDetectContextWalksUpward(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), `
module example.com/upward
require github.com/go-chi/chi/v5 v5.0.0
`)

	subdir := filepath.Join(root, "cmd", "server")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("mkdir subdir: %v", err)
	}

	ctx := DetectContext(subdir)
	if ctx == nil {
		t.Fatal("expected upward detection to find go.mod")
	}
	if ctx.RootDir != root {
		t.Fatalf("expected root %q, got %q", root, ctx.RootDir)
	}
}

func TestDetectContextReturnsNilWithoutManifest(t *testing.T) {
	if ctx := DetectContext(t.TempDir()); ctx != nil {
		t.Fatalf("expected nil context, got %#v", ctx)
	}
}

func TestDetectContextCapsDependenciesAt15(t *testing.T) {
	dir := t.TempDir()
	var deps []string
	for i := 0; i < 30; i++ {
		deps = append(deps, `"`+strings.ToLower(string(rune('a'+(i%26))))+strings.Repeat("x", i)+`":"1.0.0"`)
	}
	writeFile(t, filepath.Join(dir, "package.json"), `{"dependencies":{`+strings.Join(deps, ",")+`}}`)

	ctx := DetectContext(dir)
	if ctx == nil {
		t.Fatal("expected node context")
	}
	if len(ctx.Dependencies) > 15 {
		t.Fatalf("expected <=15 deps, got %d", len(ctx.Dependencies))
	}
}

func TestDetectContextStopsAfterTenLevels(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "go.mod"), `
module example.com/deep
require github.com/go-chi/chi/v5 v5.0.0
`)

	level := root
	for i := 0; i < 11; i++ {
		level = filepath.Join(level, "nested")
		if err := os.MkdirAll(level, 0o755); err != nil {
			t.Fatalf("mkdir level %d: %v", i, err)
		}
	}

	if ctx := DetectContext(level); ctx != nil {
		t.Fatalf("expected nil context after max walk depth, got %#v", ctx)
	}
}

func TestDescriptionIsHumanReadable(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), `
module example.com/desc
require (
    github.com/go-chi/chi/v5 v5.0.0
    github.com/jmoiron/sqlx v1.3.0
)
`)

	ctx := DetectContext(dir)
	if ctx == nil {
		t.Fatal("expected context")
	}
	if !strings.Contains(ctx.Description, "Go project using chi, sqlx") {
		t.Fatalf("unexpected description: %q", ctx.Description)
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir parent: %v", err)
	}
	if err := os.WriteFile(path, []byte(strings.TrimSpace(body)+"\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func containsAll(haystack []string, needles ...string) bool {
	seen := make(map[string]struct{}, len(haystack))
	for _, item := range haystack {
		seen[item] = struct{}{}
	}
	for _, needle := range needles {
		if _, ok := seen[needle]; !ok {
			return false
		}
	}
	return true
}
