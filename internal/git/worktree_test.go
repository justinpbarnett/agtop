package git

import (
	"fmt"
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

func TestWorktreeMergeGoldenFileConflict(t *testing.T) {
	repo := initTestRepo(t)
	wm := NewWorktreeManager(repo)

	// Create a golden file in testdata/ on main
	goldenDir := filepath.Join(repo, "internal", "ui", "testdata")
	if err := os.MkdirAll(goldenDir, 0o755); err != nil {
		t.Fatalf("mkdir testdata: %v", err)
	}
	goldenFile := filepath.Join(goldenDir, "TestSnapshot.golden")
	if err := os.WriteFile(goldenFile, []byte("main golden content"), 0o644); err != nil {
		t.Fatalf("write golden: %v", err)
	}
	gitCommit(t, repo, "add golden file")

	// Create worktree and diverge the golden file
	wtPath, _, err := wm.Create("030")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	wtGoldenFile := filepath.Join(wtPath, "internal", "ui", "testdata", "TestSnapshot.golden")
	if err := os.WriteFile(wtGoldenFile, []byte("worktree golden content"), 0o644); err != nil {
		t.Fatalf("write worktree golden: %v", err)
	}
	gitCommit(t, wtPath, "update golden in worktree")

	// Diverge golden file on main too
	if err := os.WriteFile(goldenFile, []byte("different main golden content"), 0o644); err != nil {
		t.Fatalf("write main golden: %v", err)
	}
	gitCommit(t, repo, "update golden in main")

	// Merge should succeed — golden file conflict auto-resolved
	result, err := wm.MergeWithOptions("030", MergeOptions{})
	if err != nil {
		t.Fatalf("MergeWithOptions should succeed for golden-only conflict: %v", err)
	}

	if len(result.GoldenFilesResolved) == 0 {
		t.Error("expected GoldenFilesResolved to be non-empty")
	}

	// Verify the branch version was taken
	content, err := os.ReadFile(goldenFile)
	if err != nil {
		t.Fatalf("read golden after merge: %v", err)
	}
	if string(content) != "worktree golden content" {
		t.Errorf("golden content = %q, want %q", content, "worktree golden content")
	}
}

func TestWorktreeMergeMixedConflict(t *testing.T) {
	repo := initTestRepo(t)
	wm := NewWorktreeManager(repo)

	// Create both a golden file and a regular file on main
	goldenDir := filepath.Join(repo, "internal", "ui", "testdata")
	if err := os.MkdirAll(goldenDir, 0o755); err != nil {
		t.Fatalf("mkdir testdata: %v", err)
	}
	if err := os.WriteFile(filepath.Join(goldenDir, "Test.golden"), []byte("golden v1"), 0o644); err != nil {
		t.Fatalf("write golden: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "code.go"), []byte("package main\n// v1\n"), 0o644); err != nil {
		t.Fatalf("write code: %v", err)
	}
	gitCommit(t, repo, "add golden and code files")

	wtPath, _, err := wm.Create("031")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Diverge both files in worktree
	if err := os.WriteFile(filepath.Join(wtPath, "internal", "ui", "testdata", "Test.golden"), []byte("golden v2-wt"), 0o644); err != nil {
		t.Fatalf("write worktree golden: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wtPath, "code.go"), []byte("package main\n// v2-wt\n"), 0o644); err != nil {
		t.Fatalf("write worktree code: %v", err)
	}
	gitCommit(t, wtPath, "update golden and code in worktree")

	// Diverge both files on main
	if err := os.WriteFile(filepath.Join(goldenDir, "Test.golden"), []byte("golden v2-main"), 0o644); err != nil {
		t.Fatalf("write main golden: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "code.go"), []byte("package main\n// v2-main\n"), 0o644); err != nil {
		t.Fatalf("write main code: %v", err)
	}
	gitCommit(t, repo, "update golden and code in main")

	// Merge should fail — non-golden conflict blocks it
	_, err = wm.MergeWithOptions("031", MergeOptions{})
	if err == nil {
		t.Fatal("MergeWithOptions should fail when non-golden conflicts exist")
	}

	// Verify repo is clean after abort
	cmd := exec.Command("git", "status", "--porcelain", "-uno")
	cmd.Dir = repo
	out, _ := cmd.Output()
	if len(out) > 0 {
		t.Errorf("repo not clean after aborted merge: %s", out)
	}
}

func TestWorktreeMergeGoldenUpdateCommand(t *testing.T) {
	repo := initTestRepo(t)
	wm := NewWorktreeManager(repo)

	// Create a golden file on main
	goldenDir := filepath.Join(repo, "internal", "ui", "testdata")
	if err := os.MkdirAll(goldenDir, 0o755); err != nil {
		t.Fatalf("mkdir testdata: %v", err)
	}
	goldenFile := filepath.Join(goldenDir, "Test.golden")
	if err := os.WriteFile(goldenFile, []byte("v1"), 0o644); err != nil {
		t.Fatalf("write golden: %v", err)
	}
	gitCommit(t, repo, "add golden")

	wtPath, _, err := wm.Create("032")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Diverge golden in worktree
	if err := os.WriteFile(filepath.Join(wtPath, "internal", "ui", "testdata", "Test.golden"), []byte("v2-wt"), 0o644); err != nil {
		t.Fatalf("write wt golden: %v", err)
	}
	gitCommit(t, wtPath, "update golden in worktree")

	// Diverge golden on main
	if err := os.WriteFile(goldenFile, []byte("v2-main"), 0o644); err != nil {
		t.Fatalf("write main golden: %v", err)
	}
	gitCommit(t, repo, "update golden in main")

	// Use a golden update command that writes a known value to the golden file
	updateCmd := fmt.Sprintf("echo -n 'regenerated' > '%s'", filepath.Join("internal", "ui", "testdata", "Test.golden"))

	result, err := wm.MergeWithOptions("032", MergeOptions{GoldenUpdateCommand: updateCmd})
	if err != nil {
		t.Fatalf("MergeWithOptions with update command: %v", err)
	}

	if len(result.GoldenFilesResolved) == 0 {
		t.Error("expected golden files to be resolved")
	}

	// Verify the golden update command's output was committed
	content, err := os.ReadFile(goldenFile)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if string(content) != "regenerated" {
		t.Errorf("golden content = %q, want %q", content, "regenerated")
	}

	// Verify repo is clean (update was committed)
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repo
	out, _ := cmd.Output()
	if len(out) > 0 {
		t.Errorf("repo not clean after golden update: %s", out)
	}
}

func TestRunGoldenUpdate(t *testing.T) {
	repo := initTestRepo(t)
	wm := NewWorktreeManager(repo)

	// Create and commit a file
	if err := os.WriteFile(filepath.Join(repo, "data.txt"), []byte("original"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	gitCommit(t, repo, "add data")

	// RunGoldenUpdate with a command that modifies the file
	err := wm.RunGoldenUpdate("echo -n 'updated' > data.txt")
	if err != nil {
		t.Fatalf("RunGoldenUpdate: %v", err)
	}

	// File should be updated and committed (amended)
	content, err := os.ReadFile(filepath.Join(repo, "data.txt"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(content) != "updated" {
		t.Errorf("content = %q, want %q", content, "updated")
	}

	// Repo should be clean
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repo
	out, _ := cmd.Output()
	if len(out) > 0 {
		t.Errorf("repo not clean: %s", out)
	}
}

func TestRunGoldenUpdateNoChanges(t *testing.T) {
	repo := initTestRepo(t)
	wm := NewWorktreeManager(repo)

	// RunGoldenUpdate with a no-op command — should not error
	err := wm.RunGoldenUpdate("true")
	if err != nil {
		t.Fatalf("RunGoldenUpdate no-op: %v", err)
	}
}

func TestIsGoldenFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"internal/ui/testdata/TestSnapshot.golden", true},
		{"internal/ui/panels/testdata/TestRun.golden", true},
		{"testdata/Test.golden", true},
		{"foo/testdata/bar/baz.golden", true},
		{"internal/ui/panels/runlist.go", false},
		{"testdata/config.json", false},
		{"foo.golden", false},
		{"internal/testdata.go", false},
	}
	for _, tt := range tests {
		if got := IsGoldenFile(tt.path); got != tt.want {
			t.Errorf("isGoldenFile(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

// gitCommit stages all changes and commits with the given message.
func gitCommit(t *testing.T, dir, msg string) {
	t.Helper()
	for _, args := range [][]string{
		{"git", "add", "-A"},
		{"git", "commit", "-m", msg},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s: %v", args, out, err)
		}
	}
}

func TestWorktreeMergeRebasesBeforeMerge(t *testing.T) {
	repo := initTestRepo(t)
	wm := NewWorktreeManager(repo)

	// Create a file on main before creating the worktree
	if err := os.WriteFile(filepath.Join(repo, "base.txt"), []byte("base"), 0o644); err != nil {
		t.Fatalf("write base: %v", err)
	}
	gitCommit(t, repo, "add base file")

	// Create worktree (branches from current main)
	wtPath, _, err := wm.Create("040")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Make a change on the worktree branch
	if err := os.WriteFile(filepath.Join(wtPath, "feature.txt"), []byte("feature work"), 0o644); err != nil {
		t.Fatalf("write feature: %v", err)
	}
	gitCommit(t, wtPath, "add feature file")

	// Move main forward with a non-conflicting change
	if err := os.WriteFile(filepath.Join(repo, "other.txt"), []byte("other work"), 0o644); err != nil {
		t.Fatalf("write other: %v", err)
	}
	gitCommit(t, repo, "add other file on main")

	// MergeWithOptions should succeed — rebase makes it a fast-forward
	if _, err := wm.MergeWithOptions("040", MergeOptions{}); err != nil {
		t.Fatalf("MergeWithOptions should succeed after rebase: %v", err)
	}

	// Verify both files exist in main repo
	for _, name := range []string{"feature.txt", "other.txt"} {
		if _, err := os.Stat(filepath.Join(repo, name)); os.IsNotExist(err) {
			t.Errorf("%s not found in main repo after merge", name)
		}
	}
}

func TestWorktreeMergeDirtyWorkingTree(t *testing.T) {
	repo := initTestRepo(t)
	wm := NewWorktreeManager(repo)

	// Create a file on main
	if err := os.WriteFile(filepath.Join(repo, "shared.txt"), []byte("original"), 0o644); err != nil {
		t.Fatalf("write shared: %v", err)
	}
	gitCommit(t, repo, "add shared file")

	// Create worktree and modify the shared file
	wtPath, _, err := wm.Create("050")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wtPath, "shared.txt"), []byte("worktree version"), 0o644); err != nil {
		t.Fatalf("write worktree shared: %v", err)
	}
	gitCommit(t, wtPath, "update shared in worktree")

	// Make an uncommitted change to the same file on main (dirty working tree)
	if err := os.WriteFile(filepath.Join(repo, "shared.txt"), []byte("dirty local edit"), 0o644); err != nil {
		t.Fatalf("write dirty: %v", err)
	}

	// Merge should succeed — dirty changes stashed and restored
	if err := wm.Merge("050"); err != nil {
		t.Fatalf("Merge with dirty working tree should succeed: %v", err)
	}

	// The merged content should be from the worktree branch
	content, err := os.ReadFile(filepath.Join(repo, "shared.txt"))
	if err != nil {
		t.Fatalf("read shared: %v", err)
	}
	if string(content) != "worktree version" {
		t.Errorf("shared.txt = %q, want %q", content, "worktree version")
	}
}

func TestWorktreeMergeDirtyUnrelatedFile(t *testing.T) {
	repo := initTestRepo(t)
	wm := NewWorktreeManager(repo)

	// Create worktree and add a file
	wtPath, _, err := wm.Create("051")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wtPath, "feature.txt"), []byte("feature"), 0o644); err != nil {
		t.Fatalf("write feature: %v", err)
	}
	gitCommit(t, wtPath, "add feature")

	// Create a dirty unrelated file on main
	if err := os.WriteFile(filepath.Join(repo, "unrelated.txt"), []byte("work in progress"), 0o644); err != nil {
		t.Fatalf("write unrelated: %v", err)
	}

	// Merge should succeed and preserve the dirty file
	if err := wm.Merge("051"); err != nil {
		t.Fatalf("Merge with dirty unrelated file: %v", err)
	}

	// Feature file should exist from the merge
	if _, err := os.Stat(filepath.Join(repo, "feature.txt")); os.IsNotExist(err) {
		t.Error("feature.txt not found after merge")
	}

	// Unrelated dirty file should still be present
	content, err := os.ReadFile(filepath.Join(repo, "unrelated.txt"))
	if err != nil {
		t.Fatalf("read unrelated: %v", err)
	}
	if string(content) != "work in progress" {
		t.Errorf("unrelated.txt = %q, want %q", content, "work in progress")
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
