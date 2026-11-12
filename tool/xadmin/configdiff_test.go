package xadmin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFlattenYAML(t *testing.T) {
	data := []byte(`
app:
  name: x-admin
  features:
    - a
    - b
database:
  max_open_conns: 50
  empty: {}
`)
	flat, err := FlattenYAML(data)
	if err != nil {
		t.Fatal(err)
	}
	if flat["app.name"] != "x-admin" {
		t.Fatalf("app.name = %q", flat["app.name"])
	}
	if flat["app.features"] != "[2 items]" {
		t.Fatalf("app.features = %q", flat["app.features"])
	}
	if flat["database.max_open_conns"] != "50" {
		t.Fatalf("database.max_open_conns = %q", flat["database.max_open_conns"])
	}
	if flat["database.empty"] != "{}" {
		t.Fatalf("database.empty = %q", flat["database.empty"])
	}
}

func TestDetectConfigAdditions(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "configs"), 0o755); err != nil {
		t.Fatal(err)
	}
	user := []byte("app:\n  name: my-admin\n")
	if err := os.WriteFile(filepath.Join(dir, "configs", "config.yaml"), user, 0o644); err != nil {
		t.Fatal(err)
	}
	target := Tree{
		"configs/config.example.yaml": {Data: []byte("app:\n  name: x-admin\n  version: 1.0.0\nsecurity:\n  max_login_fails: 5\n")},
	}
	added := detectConfigAdditions(target, dir)
	if len(added) != 2 {
		t.Fatalf("added = %+v", added)
	}
	if added[0].Key != "app.version" || added[1].Key != "security.max_login_fails" {
		t.Fatalf("unexpected keys: %+v", added)
	}
}

func TestConfigRemovedKeys(t *testing.T) {
	example := []byte("app:\n  name: x-admin\n")
	user := []byte("app:\n  name: my-admin\n  legacy: true\n")
	removed, err := ConfigRemovedKeys(example, user)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 1 || removed[0].Key != "app.legacy" {
		t.Fatalf("removed = %+v", removed)
	}
}
