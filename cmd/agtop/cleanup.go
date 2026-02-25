package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/justinpbarnett/agtop/internal/config"
	gitpkg "github.com/justinpbarnett/agtop/internal/git"
	"github.com/justinpbarnett/agtop/internal/run"
)

const staleSessionAge = 7 * 24 * time.Hour

func runCleanup(cfg *config.Config, dryRun bool) error {
	projectRoot := cfg.Project.Root
	if projectRoot == "" || projectRoot == "." {
		var err error
		projectRoot, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("get working directory: %w", err)
		}
	}

	persist, err := run.NewPersistence(projectRoot)
	if err != nil {
		return fmt.Errorf("init persistence: %w", err)
	}

	sessions, err := persist.Load()
	if err != nil {
		return fmt.Errorf("load sessions: %w", err)
	}

	wt := gitpkg.NewWorktreeManagerAt(projectRoot, cfg.Project.WorktreePath)

	now := time.Now()
	removedSessions := 0
	removedWorktrees := 0

	// Track which run IDs have active sessions
	activeRunIDs := make(map[string]bool)

	for _, sf := range sessions {
		shouldRemove := false

		if sf.Run.IsTerminal() && now.Sub(sf.SavedAt) > staleSessionAge {
			shouldRemove = true
			if dryRun {
				fmt.Printf("  [dry-run] would remove stale session: %s (state=%s, age=%s)\n",
					sf.Run.ID, sf.Run.State, now.Sub(sf.SavedAt).Round(time.Hour))
			}
		} else if !sf.Run.IsTerminal() && (sf.Run.PID <= 0 || !run.IsProcessAlive(sf.Run.PID)) {
			shouldRemove = true
			if dryRun {
				fmt.Printf("  [dry-run] would remove dead session: %s (state=%s, pid=%d)\n",
					sf.Run.ID, sf.Run.State, sf.Run.PID)
			}
		}

		if shouldRemove {
			if !dryRun {
				if err := persist.Remove(sf.Run.ID); err != nil {
					fmt.Fprintf(os.Stderr, "  warning: remove session %s: %v\n", sf.Run.ID, err)
				} else {
					fmt.Printf("  removed session: %s\n", sf.Run.ID)
					removedSessions++
				}
				// Clean up log files
				for _, p := range []string{sf.StdoutLogPath, sf.StderrLogPath} {
					if p != "" {
						os.Remove(p)
					}
				}
			} else {
				removedSessions++
			}
		} else {
			activeRunIDs[sf.Run.ID] = true
		}
	}

	// Find orphaned worktrees
	worktrees, err := wt.List()
	if err != nil {
		fmt.Fprintf(os.Stderr, "  warning: list worktrees: %v\n", err)
	} else {
		for _, w := range worktrees {
			runID := filepath.Base(w.Path)
			if !activeRunIDs[runID] {
				if dryRun {
					fmt.Printf("  [dry-run] would remove orphaned worktree: %s (branch=%s)\n", runID, w.Branch)
					removedWorktrees++
				} else {
					if err := wt.Remove(runID); err != nil {
						fmt.Fprintf(os.Stderr, "  warning: remove worktree %s: %v\n", runID, err)
					} else {
						fmt.Printf("  removed worktree: %s\n", runID)
						removedWorktrees++
					}
				}
			}
		}
	}

	prefix := ""
	if dryRun {
		prefix = "[dry-run] "
	}
	fmt.Printf("\n%sRemoved %d session files, %d orphaned worktrees.\n", prefix, removedSessions, removedWorktrees)
	return nil
}
