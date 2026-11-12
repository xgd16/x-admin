package xadmin

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type fakeMerger struct {
	conflict bool
	called   bool
}

func (f *fakeMerger) Merge(_ string, local, base, upstream []byte) ([]byte, bool, error) {
	f.called = true
	if f.conflict {
		return []byte("<<<<<<< local\n"), true, nil
	}
	out := append([]byte{}, local...)
	out = append(out, upstream...)
	return out, false, nil
}

func testTree(m map[string]string) Tree {
	t := Tree{}
	for k, v := range m {
		t[k] = File{Data: []byte(v), Mode: 0o644}
	}
	return t
}

func findChange(plan *Plan, path string) (Change, bool) {
	for _, ch := range plan.Changes {
		if ch.Path == path {
			return ch, true
		}
	}
	return Change{}, false
}

func TestComputePlanCases(t *testing.T) {
	tests := []struct {
		name       string
		base       map[string]string
		target     map[string]string
		local      map[string]string
		merger     *fakeMerger
		path       string
		wantKind   ChangeKind
		wantData   string
		wantUpOnly bool
		wantAbsent bool
	}{
		{
			name:     "upstream update local pristine",
			base:     map[string]string{"a.txt": "1"},
			target:   map[string]string{"a.txt": "2"},
			local:    map[string]string{"a.txt": "1"},
			path:     "a.txt",
			wantKind: ChangeUpdate,
			wantData: "2",
		},
		{
			name:     "local change upstream pristine",
			base:     map[string]string{"a.txt": "1"},
			target:   map[string]string{"a.txt": "1"},
			local:    map[string]string{"a.txt": "x"},
			path:     "a.txt",
			wantKind: ChangeKeep,
		},
		{
			name:     "both changed identically",
			base:     map[string]string{"a.txt": "1"},
			target:   map[string]string{"a.txt": "2"},
			local:    map[string]string{"a.txt": "2"},
			path:     "a.txt",
			wantKind: ChangeSame,
		},
		{
			name:     "both changed clean merge",
			base:     map[string]string{"a.txt": "1"},
			target:   map[string]string{"a.txt": "2"},
			local:    map[string]string{"a.txt": "3"},
			merger:   &fakeMerger{},
			path:     "a.txt",
			wantKind: ChangeUpdate,
			wantData: "32",
		},
		{
			name:     "both changed conflict",
			base:     map[string]string{"a.txt": "1"},
			target:   map[string]string{"a.txt": "2"},
			local:    map[string]string{"a.txt": "3"},
			merger:   &fakeMerger{conflict: true},
			path:     "a.txt",
			wantKind: ChangeConflict,
			wantData: "<<<<<<< local\n",
		},
		{
			name:     "upstream add",
			target:   map[string]string{"b.txt": "2"},
			local:    map[string]string{},
			path:     "b.txt",
			wantKind: ChangeAdd,
			wantData: "2",
		},
		{
			name:       "upstream add collides local file",
			target:     map[string]string{"b.txt": "2"},
			local:      map[string]string{"b.txt": "3"},
			path:       "b.txt",
			wantKind:   ChangeConflict,
			wantData:   "2",
			wantUpOnly: true,
		},
		{
			name:     "upstream add identical local file",
			target:   map[string]string{"b.txt": "2"},
			local:    map[string]string{"b.txt": "2"},
			path:     "b.txt",
			wantKind: ChangeSame,
		},
		{
			name:     "upstream delete local pristine",
			base:     map[string]string{"c.txt": "1"},
			target:   map[string]string{},
			local:    map[string]string{"c.txt": "1"},
			path:     "c.txt",
			wantKind: ChangeDelete,
		},
		{
			name:     "upstream delete local modified",
			base:     map[string]string{"c.txt": "1"},
			target:   map[string]string{},
			local:    map[string]string{"c.txt": "x"},
			path:     "c.txt",
			wantKind: ChangeConflict,
		},
		{
			name:       "local delete upstream modified",
			base:       map[string]string{"d.txt": "1"},
			target:     map[string]string{"d.txt": "2"},
			local:      map[string]string{},
			path:       "d.txt",
			wantKind:   ChangeConflict,
			wantData:   "2",
			wantUpOnly: true,
		},
		{
			name:       "local delete upstream pristine",
			base:       map[string]string{"f.txt": "1"},
			target:     map[string]string{"f.txt": "1"},
			local:      map[string]string{},
			path:       "f.txt",
			wantKind:   ChangeKeep,
			wantAbsent: false,
		},
		{
			name:       "local only file ignored",
			target:     map[string]string{},
			local:      map[string]string{"mine.txt": "x"},
			path:       "mine.txt",
			wantAbsent: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			merger := tt.merger
			if merger == nil {
				merger = &fakeMerger{}
			}
			plan := ComputePlan("owner/repo", "/tmp", "v1", "aaa", "v2", "bbb",
				testTree(tt.base), testTree(tt.target), testTree(tt.local), merger)
			ch, ok := findChange(plan, tt.path)
			if tt.wantAbsent {
				if ok {
					t.Fatalf("path %q should be absent, got %+v", tt.path, ch)
				}
				return
			}
			if !ok {
				t.Fatalf("path %q not found in plan: %+v", tt.path, plan.Changes)
			}
			if ch.Kind != tt.wantKind {
				t.Fatalf("kind = %s, want %s (reason: %s)", ch.Kind, tt.wantKind, ch.Reason)
			}
			if tt.wantData != "" && string(ch.Data) != tt.wantData {
				t.Fatalf("data = %q, want %q", ch.Data, tt.wantData)
			}
			if ch.UpstreamOnly != tt.wantUpOnly {
				t.Fatalf("upstreamOnly = %v, want %v", ch.UpstreamOnly, tt.wantUpOnly)
			}
		})
	}
}

func TestComputePlanBinaryConflict(t *testing.T) {
	base := Tree{"img.bin": {Data: []byte{0, 1}, Mode: 0o644}}
	target := Tree{"img.bin": {Data: []byte{0, 2}, Mode: 0o644}}
	local := Tree{"img.bin": {Data: []byte{0, 3}, Mode: 0o644}}
	m := &fakeMerger{}
	plan := ComputePlan("o/r", "/tmp", "v1", "a", "v2", "b", base, target, local, m)
	ch, ok := findChange(plan, "img.bin")
	if !ok || ch.Kind != ChangeConflict || !ch.UpstreamOnly {
		t.Fatalf("unexpected change: %+v", ch)
	}
	if m.called {
		t.Fatal("merger should not be called for binary files")
	}
}

func TestGitMergerCleanAndConflict(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	m := GitMerger{}
	base := []byte("a\nb\nc\n")
	local := []byte("A\nb\nc\n")
	upstream := []byte("a\nb\nC\n")
	merged, conflict, err := m.Merge("x", local, base, upstream)
	if err != nil {
		t.Fatal(err)
	}
	if conflict {
		t.Fatal("expected clean merge")
	}
	if string(merged) != "A\nb\nC\n" {
		t.Fatalf("merged = %q", merged)
	}

	local2 := []byte("X\nb\nc\n")
	upstream2 := []byte("Y\nb\nc\n")
	merged, conflict, err = m.Merge("x", local2, base, upstream2)
	if err != nil {
		t.Fatal(err)
	}
	if !conflict {
		t.Fatal("expected conflict")
	}
	if !bytes.Contains(merged, []byte("<<<<<<< local")) || !bytes.Contains(merged, []byte(">>>>>>> upstream")) {
		t.Fatalf("missing conflict markers: %s", merged)
	}
}

func TestGitMergerBinary(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	m := GitMerger{}
	_, conflict, err := m.Merge("x", []byte{0, 1}, []byte{0, 0}, []byte{0, 2})
	if err != nil {
		t.Fatal(err)
	}
	if !conflict {
		t.Fatal("binary files should conflict")
	}
}

func TestFallbackMerger(t *testing.T) {
	m := FallbackMerger{}
	out, conflict, err := m.Merge("x", []byte("local"), []byte("base"), []byte("base"))
	if err != nil || conflict || !strings.Contains(string(out), "local") {
		t.Fatalf("upstream pristine should keep local: %q %v %v", out, conflict, err)
	}
	_, conflict, err = m.Merge("x", []byte("local"), []byte("base"), []byte("upstream"))
	if err != nil || !conflict {
		t.Fatalf("both changed should conflict: %v %v", conflict, err)
	}
}

func TestApplyPlanWritesConflictUpstream(t *testing.T) {
	dir := t.TempDir()
	plan := &Plan{Changes: []Change{
		{Path: "a.txt", Kind: ChangeUpdate, Data: []byte("new"), Mode: 0o644},
		{Path: "b.txt", Kind: ChangeAdd, Data: []byte("add"), Mode: 0o644},
		{Path: "c.txt", Kind: ChangeDelete},
		{Path: "d.txt", Kind: ChangeConflict, UpstreamOnly: true, Data: []byte("upstream"), Mode: 0o644},
	}}
	if err := writeWorkFile(dir, "c.txt", []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeWorkFile(dir, "d.txt", []byte("local"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ApplyPlan(plan, dir); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "a.txt")); string(b) != "new" {
		t.Fatalf("a.txt = %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "b.txt")); string(b) != "add" {
		t.Fatalf("b.txt = %q", b)
	}
	if _, err := os.Stat(filepath.Join(dir, "c.txt")); err == nil {
		t.Fatal("c.txt should be deleted")
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "d.txt")); string(b) != "local" {
		t.Fatalf("d.txt local content changed: %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "d.txt.upstream")); string(b) != "upstream" {
		t.Fatalf("d.txt.upstream = %q", b)
	}
}
