package main

import "embed"

//go:embed welcome.md
var welcomeDoc []byte

// envTemplate is .env.example, built into the binary so `mdv init` works
// without a clone of the repo.
//
//go:embed .env.example
var envTemplate string

//go:embed skills/mdv-docs
var skillFS embed.FS
