package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justinpbarnett/agtop/internal/config"
	"github.com/justinpbarnett/agtop/internal/safety"
	"github.com/justinpbarnett/agtop/internal/ui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Route subcommands
	if len(os.Args) > 1 && os.Args[1] == "init" {
		if err := runInit(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Initialize safety engine (log warnings for invalid patterns)
	_, safetyErr := safety.NewHookEngine(cfg.Safety)
	if safetyErr != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", safetyErr)
	}

	model := ui.NewApp(cfg)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if mgr := model.Manager(); mgr != nil {
		mgr.SetProgram(p)
	}

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
