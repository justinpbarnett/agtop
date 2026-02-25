package main

import (
	"fmt"

	"github.com/justinpbarnett/agtop/internal/ui/panels"
	"github.com/justinpbarnett/agtop/internal/update"
)

func runVersion(repo string) {
	fmt.Printf("agtop version %s\n", panels.Version)

	if panels.Version == "dev" {
		fmt.Println("Development build — update check skipped.")
		return
	}

	rel, err := update.CheckForUpdate(panels.Version, repo)
	if err != nil {
		fmt.Printf("Update check failed: %v\n", err)
		return
	}

	if rel != nil {
		fmt.Printf("Update available: v%s. Run \"agtop update\" to install.\n", rel.Version)
	} else {
		fmt.Println("You are up to date.")
	}
}
