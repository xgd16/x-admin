package xadmin

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, root, rel, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func initTestRepo(t *testing.T, dir string) *Git {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
		}
	}
	run("init", "-q")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "test")
	run("add", "-A")
	run("commit", "-qm", "base")
	git, err := NewGit(dir)
	if err != nil {
		t.Fatal(err)
	}
	return git
}

func TestUpgradeApplyAndRollback(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	targetDir := filepath.Join(root, "target")
	workDir := filepath.Join(root, "work")
	for _, d := range []string{baseDir, targetDir, workDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeTestFile(t, baseDir, "a.txt", "one\n")
	writeTestFile(t, baseDir, "gone.txt", "old\n")
	writeTestFile(t, baseDir, "migrations/000001_init.up.sql", "-- init\n")
	writeTestFile(t, baseDir, "migrations/000001_init.down.sql", "-- down\n")
	writeTestFile(t, targetDir, "a.txt", "two\n")
	writeTestFile(t, targetDir, "added.txt", "new\n")
	writeTestFile(t, targetDir, "migrations/000001_init.up.sql", "-- init\n")
	writeTestFile(t, targetDir, "migrations/000001_init.down.sql", "-- down\n")
	writeTestFile(t, targetDir, "migrations/000002_added.up.sql", "-- added\n")
	writeTestFile(t, targetDir, "migrations/000002_added.down.sql", "-- down\n")
	for _, rel := range []string{"a.txt", "gone.txt", "migrations/000001_init.up.sql", "migrations/000001_init.down.sql"} {
		data, err := os.ReadFile(filepath.Join(baseDir, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		writeTestFile(t, workDir, rel, string(data))
	}
	writeTestFile(t, workDir, "mine.txt", "local\n")
	git := initTestRepo(t, workDir)
	originalBranch := git.Branch()

	ctx := context.Background()
	opts := SourceOptions{
		Repo:    DefaultRepo,
		Dir:     workDir,
		FromDir: baseDir,
		ToDir:   targetDir,
		From:    "v1",
		To:      "v2",
	}
	res, plan, _, err := PrepareUpgrade(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	if plan.HasConflicts() || plan.HasMigrationErrors() {
		t.Fatalf("unexpected conflicts: %+v", plan)
	}
	if err := ExecuteUpgrade(ctx, res, plan, ApplyOptions{Yes: true, NoVerify: true}, io.Discard); err != nil {
		t.Fatal(err)
	}
	if git.Branch() == originalBranch {
		t.Fatal("expected a new branch")
	}
	if data, _ := os.ReadFile(filepath.Join(workDir, "a.txt")); string(data) != "two\n" {
		t.Fatalf("a.txt = %q", data)
	}
	if data, _ := os.ReadFile(filepath.Join(workDir, "mine.txt")); string(data) != "local\n" {
		t.Fatalf("mine.txt = %q", data)
	}
	if _, err := os.Stat(filepath.Join(workDir, "gone.txt")); !os.IsNotExist(err) {
		t.Fatal("gone.txt should be deleted")
	}
	if _, err := os.Stat(filepath.Join(workDir, "added.txt")); err != nil {
		t.Fatal("added.txt should exist")
	}
	state, err := ReadState(workDir)
	if err != nil || state == nil || state.Ref != "v2" {
		t.Fatalf("state = %+v, err = %v", state, err)
	}
	if version, err := ReadFrameworkVersion(workDir); err != nil || version != "v2" {
		t.Fatalf("version = %q, err = %v", version, err)
	}
	if out, err := exec.Command("git", "-C", workDir, "log", "--oneline").Output(); err != nil || !strings.Contains(string(out), "chore(upgrade)") {
		t.Fatalf("expected upgrade commit: %s %v", out, err)
	}

	n, manifest, err := Rollback(git, workDir, mustLatestBackup(t, git))
	if err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatal("rollback restored nothing")
	}
	if manifest.OriginalBranch != originalBranch {
		t.Fatalf("original branch = %q, want %q", manifest.OriginalBranch, originalBranch)
	}
	if data, _ := os.ReadFile(filepath.Join(workDir, "a.txt")); string(data) != "one\n" {
		t.Fatalf("a.txt after rollback = %q", data)
	}
	if _, err := os.Stat(filepath.Join(workDir, "added.txt")); !os.IsNotExist(err) {
		t.Fatal("added.txt should be removed by rollback")
	}
	if data, _ := os.ReadFile(filepath.Join(workDir, "gone.txt")); string(data) != "old\n" {
		t.Fatalf("gone.txt after rollback = %q", data)
	}
	if _, err := os.Stat(filepath.Join(workDir, StatePath)); !os.IsNotExist(err) {
		t.Fatal("state file should be removed by rollback")
	}
	if _, err := os.Stat(filepath.Join(workDir, VersionPath)); !os.IsNotExist(err) {
		t.Fatal("version file should be removed by rollback")
	}
}

func TestUpgradeApplyConflict(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	baseDir := filepath.Join(root, "base")
	targetDir := filepath.Join(root, "target")
	workDir := filepath.Join(root, "work")
	for _, d := range []string{baseDir, targetDir, workDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeTestFile(t, baseDir, "a.txt", "a\nb\nc\n")
	writeTestFile(t, targetDir, "a.txt", "a\nUP\nc\n")
	writeTestFile(t, workDir, "a.txt", "a\nLOCAL\nc\n")
	initTestRepo(t, workDir)

	ctx := context.Background()
	opts := SourceOptions{Repo: DefaultRepo, Dir: workDir, FromDir: baseDir, ToDir: targetDir, From: "v1", To: "v2"}
	res, plan, _, err := PrepareUpgrade(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	err = ExecuteUpgrade(ctx, res, plan, ApplyOptions{Yes: true, NoVerify: true}, io.Discard)
	if !errors.Is(err, ErrConflicts) {
		t.Fatalf("expected ErrConflicts, got %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(workDir, "a.txt"))
	if !strings.Contains(string(data), "<<<<<<< local") {
		t.Fatalf("missing conflict markers: %s", data)
	}
	out, _ := exec.Command("git", "-C", workDir, "log", "--oneline").Output()
	if strings.Contains(string(out), "chore(upgrade)") {
		t.Fatal("conflicting upgrade should not commit")
	}
}

func mustLatestBackup(t *testing.T, git *Git) string {
	t.Helper()
	dir, err := LatestBackup(git)
	if err != nil {
		t.Fatal(err)
	}
	return dir
}
