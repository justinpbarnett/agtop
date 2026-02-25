package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justinpbarnett/agtop/internal/config"
	"github.com/justinpbarnett/agtop/internal/safety"
	"github.com/justinpbarnett/agtop/internal/setup"
	"github.com/justinpbarnett/agtop/internal/ui"
	"github.com/justinpbarnett/agtop/internal/ui/panels"
	"github.com/justinpbarnett/agtop/internal/update"
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
			runtimeFlag := flagValue(os.Args[2:], "--runtime")
			if err := runInit(cfg, useAI, runtimeFlag); err != nil {
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

	// Make the embedded config template available to the setup package
	// so in-process init (from the onboarding modal) can generate agtop.toml.
	setup.DefaultConfig = defaultConfig

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

func flagValue(args []string, flag string) string {
	for i, a := range args {
		if a == flag && i+1 < len(args) {
			return args[i+1]
		}
		if strings.HasPrefix(a, flag+"=") {
			return strings.TrimPrefix(a, flag+"=")
		}
	}
	return ""
}
