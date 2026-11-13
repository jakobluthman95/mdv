package main

import (
	_ "embed"
	"errors"
	"os"
	"path/filepath"
	"runtime/debug"
)

// version is set at build time by GoReleaser and make install
// (-ldflags "-X main.version=..."). go install builds fall back to the module
// version Go records in the binary.
var version = ""

func mdvVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

//go:embed welcome.md
var welcomeDoc []byte

// ensureDocsDir creates the docs folder if needed. A newly created folder gets
// a welcome doc, so the first `mdv` has something to show.
func ensureDocsDir(dir string) error {
	_, err := os.Stat(dir)
	if err == nil {
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "welcome.md"), welcomeDoc, 0o644)
}
