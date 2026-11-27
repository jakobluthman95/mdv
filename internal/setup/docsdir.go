package setup

import (
	"errors"
	"os"
	"path/filepath"
)

var welcomeDoc []byte

func SetWelcome(b []byte) { welcomeDoc = b }

// EnsureDocsDir creates the docs folder if needed. A newly created folder gets
// a welcome doc, so the first `mdv` has something to show.
func EnsureDocsDir(dir string) error {
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
