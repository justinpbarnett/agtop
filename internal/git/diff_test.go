package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiffGeneratorDiff(t *testing.T) {
	repo := initTestRepo(t)

	// Create a branch with a change
	cmds := [][]string{
		{"git", "checkout", "-b", "test-branch"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("setup %v: %s: %v", args, out, err)
		}
	}

	// Write a file and commit
	if err := os.WriteFile(filepath.Join(repo, "hello.txt"), []byte("hello\n"), 0o644); err != nil {
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

	dg := NewDiffGenerator(repo)

	diff, err := dg.Diff("test-branch")
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}

	if !strings.Contains(diff, "hello.txt") {
		t.Error("diff output should contain hello.txt")
	}
}

func TestDiffGeneratorDiffStat(t *testing.T) {
	repo := initTestRepo(t)

	cmds := [][]string{
		{"git", "checkout", "-b", "stat-branch"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("setup %v: %s: %v", args, out, err)
		}
	}

	if err := os.WriteFile(filepath.Join(repo, "world.txt"), []byte("world\n"), 0o644); err != nil {
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

	dg := NewDiffGenerator(repo)

	stat, err := dg.DiffStat("stat-branch")
	if err != nil {
		t.Fatalf("DiffStat: %v", err)
	}

	if !strings.Contains(stat, "world.txt") {
		t.Error("diffstat output should contain world.txt")
	}
}
