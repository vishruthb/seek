package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareAttachedFilesLoadsContextAndCleansQuery(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "app.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("write app.go: %v", err)
	}

	query, files, err := prepareAttachedFiles("review @[app.go]", root)
	if err != nil {
		t.Fatalf("prepareAttachedFiles: %v", err)
	}
	if query != "review app.go" {
		t.Fatalf("expected cleaned query, got %q", query)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 attached file, got %d", len(files))
	}
	if files[0].DisplayPath != "app.go" || !strings.Contains(files[0].Content, "func main() {}") {
		t.Fatalf("unexpected attached file: %#v", files[0])
	}
}

func TestPrepareAttachedFilesRejectsOutsideWorkingDir(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o644); err != nil {
		t.Fatalf("write secret.txt: %v", err)
	}

	_, _, err := prepareAttachedFiles("review @["+outsideFile+"]", root)
	if err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("expected outside-working-dir error, got %v", err)
	}
}

func TestPrepareAttachedFilesRejectsSymlinkEscapes(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	outsideFile := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o644); err != nil {
		t.Fatalf("write secret.txt: %v", err)
	}

	linkPath := filepath.Join(root, "link.txt")
	if err := os.Symlink(outsideFile, linkPath); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	_, _, err := prepareAttachedFiles("review @[link.txt]", root)
	if err == nil || !strings.Contains(err.Error(), "outside") {
		t.Fatalf("expected symlink escape error, got %v", err)
	}
}
