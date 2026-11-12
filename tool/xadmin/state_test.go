package xadmin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFrameworkVersionFile(t *testing.T) {
	dir := t.TempDir()
	if got, err := ReadFrameworkVersion(dir); err != nil || got != "" {
		t.Fatalf("missing file: got %q, err %v", got, err)
	}
	if err := WriteFrameworkVersion(dir, " v1.3.0 "); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, VersionPath))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "v1.3.0\n" {
		t.Fatalf("file content = %q", data)
	}
	if got, err := ReadFrameworkVersion(dir); err != nil || got != "v1.3.0" {
		t.Fatalf("read = %q, err %v", got, err)
	}
}

func TestStateRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := State{Repo: "xgd16/x-admin", Ref: "v1.3.0", Commit: "abc"}
	if err := WriteState(dir, want); err != nil {
		t.Fatal(err)
	}
	got, err := ReadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Repo != want.Repo || got.Ref != want.Ref || got.Commit != want.Commit {
		t.Fatalf("state = %+v", got)
	}
	if got.UpdatedAt.IsZero() {
		t.Fatal("updated_at should be set")
	}
}
