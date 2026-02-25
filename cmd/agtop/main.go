package main

import (
	"fmt"
	"io"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justinpbarnett/agtop/internal/config"
	"github.com/justinpbarnett/agtop/internal/safety"
	"github.com/justinpbarnett/agtop/internal/ui"
	"github.com/justinpbarnett/agtop/internal/update"
	"github.com/justinpbarnett/agtop/internal/ui/panels"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Route subcommands
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "init":
			useAI := !hasFlag(os.Args[2:], "--no-ai")
			if err := runInit(cfg, useAI); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			return
		case "cleanup":
			dryRun := len(os.Args) > 2 && os.Args[2] == "--dry-run"
			if err := runCleanup(cfg, dryRun); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			return
		case "version":
			runVersion(cfg.Update.Repo)
			return
		case "update":
			if panels.Version == "dev" {
				fmt.Fprintln(os.Stderr, "Development build — cannot self-update. Install from a release first.")
				os.Exit(1)
			}
			latest, err := update.CheckForUpdate(panels.Version, cfg.Update.Repo)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			if latest == nil {
				fmt.Println("Already up to date.")
				return
			}
			fmt.Printf("Updating agtop to v%s...\n", latest.Version)
			if _, err := update.Apply(panels.Version, cfg.Update.Repo); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Updated to v%s. Restart agtop to use the new version.\n", latest.Version)
			return
		}
	}

	// Initialize safety engine (log warnings for invalid patterns)
	_, safetyErr := safety.NewHookEngine(cfg.Safety)
	if safetyErr != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", safetyErr)
	}

	// Silence internal logging so no log.Printf output leaks to the terminal
	// and disrupts the TUI layout during normal operation.
	log.SetOutput(io.Discard)

	model := ui.NewApp(cfg)
	p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())

	if mgr := model.Manager(); mgr != nil {
		mgr.SetProgram(p)
	}

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}
