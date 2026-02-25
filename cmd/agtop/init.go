package main

import (
	"github.com/justinpbarnett/agtop/internal/config"
	"github.com/justinpbarnett/agtop/internal/detect"
	"github.com/justinpbarnett/agtop/internal/setup"
)

func runInit(cfg *config.Config, useAI bool, runtimeFlag string) error {
	setup.DefaultConfig = defaultConfig

	runtime := runtimeFlag
	if runtime == "" {
		detected := detect.DetectRuntime()
		if detected != "" {
			runtime = detected
		} else {
			runtime = "claude"
		}
	}

	return setup.Run(cfg, setup.Options{
		Runtime: runtime,
		UseAI:   useAI,
	})
}
