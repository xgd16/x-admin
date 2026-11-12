package xadmin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFixModulePath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/myapp\n\ngo 1.27\n\ntool x-admin/cmd/xadmin\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "cmd", "xadmin"), 0o755); err != nil {
		t.Fatal(err)
	}
	mainSrc := "package main\n\nimport \"x-admin/tool/xadmin\"\n\nfunc main() {}\n"
	if err := os.WriteFile(filepath.Join(dir, "cmd", "xadmin", "main.go"), []byte(mainSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := FixModulePath(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("expected changes")
	}
	goMod, _ := os.ReadFile(filepath.Join(dir, "go.mod"))
	if !strings.Contains(string(goMod), "tool example.com/myapp/cmd/xadmin") {
		t.Fatalf("go.mod not fixed:\n%s", goMod)
	}
	mainData, _ := os.ReadFile(filepath.Join(dir, "cmd", "xadmin", "main.go"))
	if !strings.Contains(string(mainData), `"example.com/myapp/tool/xadmin"`) {
		t.Fatalf("main.go not fixed:\n%s", mainData)
	}
	changed, err = FixModulePath(dir)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("second run should be a no-op")
	}
}

func TestFixModulePathDefaultModule(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module x-admin\n\ngo 1.27\n\ntool x-admin/cmd/xadmin\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := FixModulePath(dir)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("default module should not change")
	}
}
