package xadmin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadTreeSkipsIgnoredAndUserFiles(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"README.md":                   "# hi",
		"configs/config.example.yaml": "app: {}",
		"configs/config.yaml":         "app: secret",
		".env":                        "SECRET=1",
		".env.example":                "SECRET=",
		"node_modules/pkg/index.js":   "ignored",
		"frontend/dist/index.html":    "ignored",
		"data/uploads/file.txt":       "ignored",
		".git/config":                 "ignored",
		"internal/main.go":            "package main",
	}
	for rel, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// 符号链接不应被读取。
	_ = os.Symlink(filepath.Join(dir, "README.md"), filepath.Join(dir, "link.md"))

	tree, err := LoadTree(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"README.md", "configs/config.example.yaml", ".env.example", "internal/main.go"}
	for _, rel := range want {
		if _, ok := tree[rel]; !ok {
			t.Errorf("missing %s", rel)
		}
	}
	unwanted := []string{
		"configs/config.yaml", ".env", "node_modules/pkg/index.js",
		"frontend/dist/index.html", "data/uploads/file.txt", ".git/config", "link.md",
	}
	for _, rel := range unwanted {
		if _, ok := tree[rel]; ok {
			t.Errorf("should skip %s", rel)
		}
	}
}

func TestSafeJoinRejectsEscape(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{"../evil", "a/../../evil", "a/b/../../../evil"} {
		if _, err := safeJoin(root, rel); err == nil {
			t.Errorf("safeJoin(%q) should fail", rel)
		}
	}
	got, err := safeJoin(root, "internal/main.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, root) {
		t.Fatalf("safeJoin returned %q outside %q", got, root)
	}
}
