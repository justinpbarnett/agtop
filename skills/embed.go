package skills

import "embed"

// FS contains the built-in skill files embedded at compile time.
//
//go:embed */SKILL.md */references/*
var FS embed.FS
