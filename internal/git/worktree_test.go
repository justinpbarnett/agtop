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
