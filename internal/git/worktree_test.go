package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "init"},
		{"git", "branch", "-M", "main"},
	}

	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("setup %v: %s: %v", args, out, err)
		}
	}
	return dir
}

func TestWorktreeCreate(t *testing.T) {
	repo := initTestRepo(t)
	wm := NewWorktreeManager(repo)

	wtPath, branch, err := wm.Create("001")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if branch != "agtop/001" {
		t.Errorf("branch = %q, want %q", branch, "agtop/001")
	}

	expectedPath := filepath.Join(repo, ".agtop", "worktrees", "001")
	if wtPath != expectedPath {
		t.Errorf("wtPath = %q, want %q", wtPath, expectedPath)
	}

	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Error("worktree directory does not exist")
	}

	// Verify the branch was created
	cmd := exec.Command("git", "branch", "--list", "agtop/001")
	cmd.Dir = repo
	out, _ := cmd.Output()
	if len(out) == 0 {
		t.Error("branch agtop/001 was not created")
	}
}

func TestWorktreeRemove(t *testing.T) {
	repo := initTestRepo(t)
	wm := NewWorktreeManager(repo)

	_, _, err := wm.Create("002")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := wm.Remove("002"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	wtPath := filepath.Join(repo, ".agtop", "worktrees", "002")
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Error("worktree directory still exists after remove")
	}

	cmd := exec.Command("git", "branch", "--list", "agtop/002")
	cmd.Dir = repo
	out, _ := cmd.Output()
	if len(out) > 0 {
		t.Error("branch agtop/002 still exists after remove")
	}
}

func TestWorktreeRemoveIdempotent(t *testing.T) {
	repo := initTestRepo(t)
	wm := NewWorktreeManager(repo)

	// Remove a worktree that was never created — should not error
	if err := wm.Remove("nonexistent"); err != nil {
		t.Fatalf("Remove nonexistent: %v", err)
	}
}

func TestWorktreeList(t *testing.T) {
	repo := initTestRepo(t)
	wm := NewWorktreeManager(repo)

	_, _, _ = wm.Create("010")
	_, _, _ = wm.Create("011")

	list, err := wm.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(list) != 2 {
		t.Fatalf("List returned %d worktrees, want 2", len(list))
	}

	branches := map[string]bool{}
	for _, wt := range list {
		branches[wt.Branch] = true
	}
	if !branches["agtop/010"] || !branches["agtop/011"] {
		t.Errorf("expected branches agtop/010 and agtop/011, got %v", branches)
	}
}

func TestWorktreeMerge(t *testing.T) {
	repo := initTestRepo(t)
	wm := NewWorktreeManager(repo)

	wtPath, _, err := wm.Create("020")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Commit a file in the worktree branch
	if err := os.WriteFile(filepath.Join(wtPath, "hello.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	for _, args := range [][]string{
		{"git", "add", "hello.txt"},
		{"git", "commit", "-m", "add hello"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = wtPath
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s: %v", args, out, err)
		}
	}

	// Merge back into main
	if err := wm.Merge("020"); err != nil {
		t.Fatalf("Merge: %v", err)
	}

	// Verify file exists in main repo
	if _, err := os.Stat(filepath.Join(repo, "hello.txt")); os.IsNotExist(err) {
		t.Error("hello.txt not found in main repo after merge")
	}
}

func TestWorktreeMergeConflict(t *testing.T) {
	repo := initTestRepo(t)
	wm := NewWorktreeManager(repo)

	// Create a file in main
	if err := os.WriteFile(filepath.Join(repo, "conflict.txt"), []byte("main content"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	for _, args := range [][]string{
		{"git", "add", "conflict.txt"},
		{"git", "commit", "-m", "add conflict.txt in main"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s: %v", args, out, err)
		}
	}

	wtPath, _, err := wm.Create("021")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Make diverging change in worktree
	if err := os.WriteFile(filepath.Join(wtPath, "conflict.txt"), []byte("worktree content"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	for _, args := range [][]string{
		{"git", "add", "conflict.txt"},
		{"git", "commit", "-m", "change conflict.txt in worktree"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = wtPath
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s: %v", args, out, err)
		}
	}

	// Make diverging change in main
	if err := os.WriteFile(filepath.Join(repo, "conflict.txt"), []byte("different main content"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	for _, args := range [][]string{
		{"git", "add", "conflict.txt"},
		{"git", "commit", "-m", "change conflict.txt in main"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s: %v", args, out, err)
		}
	}

	// Merge should fail
	if err := wm.Merge("021"); err == nil {
		t.Fatal("Merge should have failed due to conflict")
	}

	// Verify repo is clean (merge was aborted) — exclude untracked files
	// since the .agtop worktree directory is expected
	cmd := exec.Command("git", "status", "--porcelain", "-uno")
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git status: %v", err)
	}
	if len(out) > 0 {
		t.Errorf("repo not clean after aborted merge: %s", out)
	}
}

func TestWorktreeExists(t *testing.T) {
	repo := initTestRepo(t)
	wm := NewWorktreeManager(repo)

	if wm.Exists("003") {
		t.Error("Exists returned true for non-existent worktree")
	}

	_, _, _ = wm.Create("003")
	if !wm.Exists("003") {
		t.Error("Exists returned false for existing worktree")
	}
}
