package xadmin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindProjectRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/app\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(root, "internal", "service")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := findProjectRoot(nested); got != root {
		t.Fatalf("findProjectRoot(%q) = %q, want %q", nested, got, root)
	}
}

func TestFindProjectRootWithoutGoMod(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := findProjectRoot(nested); got != nested {
		t.Fatalf("expected %q, got %q", nested, got)
	}
}
