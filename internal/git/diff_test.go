package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiffGeneratorDiffUncommitted(t *testing.T) {
	repo := initTestRepo(t)

	// Create a tracked file on main so we can modify it in the worktree
	if err := os.WriteFile(filepath.Join(repo, "hello.txt"), []byte("original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"git", "add", "hello.txt"},
		{"git", "commit", "-m", "add hello"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("setup %v: %s: %v", args, out, err)
		}
	}

	// Create a worktree and modify the file without committing
	wtDir := filepath.Join(t.TempDir(), "wt")
	cmd := exec.Command("git", "worktree", "add", wtDir, "-b", "test-branch")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("worktree add: %s: %v", out, err)
	}

	if err := os.WriteFile(filepath.Join(wtDir, "hello.txt"), []byte("modified\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	dg := NewDiffGenerator(repo)

	diff, err := dg.Diff(wtDir)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}

	if !strings.Contains(diff, "hello.txt") {
		t.Error("diff output should contain hello.txt for uncommitted worktree changes")
	}
	if !strings.Contains(diff, "modified") {
		t.Error("diff output should contain the modified content")
	}
}

func TestDiffGeneratorDiffCommitted(t *testing.T) {
	repo := initTestRepo(t)

	// Create a worktree and commit a new file
	wtDir := filepath.Join(t.TempDir(), "wt")
	cmd := exec.Command("git", "worktree", "add", wtDir, "-b", "committed-branch")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("worktree add: %s: %v", out, err)
	}

	if err := os.WriteFile(filepath.Join(wtDir, "committed.txt"), []byte("committed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"git", "add", "committed.txt"},
		{"git", "commit", "-m", "add committed"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = wtDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("setup %v: %s: %v", args, out, err)
		}
	}

	dg := NewDiffGenerator(repo)

	diff, err := dg.Diff(wtDir)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}

	if !strings.Contains(diff, "committed.txt") {
		t.Error("diff output should contain committed.txt for committed worktree changes")
	}
}

func TestDiffGeneratorDiffStat(t *testing.T) {
	repo := initTestRepo(t)

	// Create a tracked file on main
	if err := os.WriteFile(filepath.Join(repo, "world.txt"), []byte("original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"git", "add", "world.txt"},
		{"git", "commit", "-m", "add world"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("setup %v: %s: %v", args, out, err)
		}
	}

	// Create worktree and modify the file
	wtDir := filepath.Join(t.TempDir(), "wt")
	cmd := exec.Command("git", "worktree", "add", wtDir, "-b", "stat-branch")
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("worktree add: %s: %v", out, err)
	}

	if err := os.WriteFile(filepath.Join(wtDir, "world.txt"), []byte("modified\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	dg := NewDiffGenerator(repo)

	stat, err := dg.DiffStat(wtDir)
	if err != nil {
		t.Fatalf("DiffStat: %v", err)
	}

	if !strings.Contains(stat, "world.txt") {
		t.Error("diffstat output should contain world.txt")
	}
}
