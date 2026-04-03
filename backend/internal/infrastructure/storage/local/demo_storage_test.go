package local

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDemoStorage_Save_CreatesFileAndWritesContent(t *testing.T) {
	baseDir := filepath.Join(t.TempDir(), "uploads")
	storage := NewDemoStorage(baseDir)

	path, err := storage.Save(strings.NewReader("hello-demo"), "match.dem")
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}

	if filepath.Dir(path) != baseDir {
		t.Fatalf("expected file under base dir %q, got %q", baseDir, path)
	}

	if !strings.HasSuffix(path, "_match.dem") {
		t.Fatalf("expected generated name to end with _match.dem, got %q", path)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read stored file: %v", err)
	}

	if string(content) != "hello-demo" {
		t.Fatalf("unexpected content: %q", string(content))
	}
}

func TestDemoStorage_Save_StripsPathTraversalFromFilename(t *testing.T) {
	baseDir := filepath.Join(t.TempDir(), "uploads")
	storage := NewDemoStorage(baseDir)

	path, err := storage.Save(strings.NewReader("demo"), "../../evil.dem")
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}

	if filepath.Dir(path) != baseDir {
		t.Fatalf("expected file under base dir %q, got %q", baseDir, path)
	}

	if !strings.HasSuffix(path, "_evil.dem") {
		t.Fatalf("expected sanitized filename to end with _evil.dem, got %q", path)
	}
}
