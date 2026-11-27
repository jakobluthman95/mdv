package docs

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func Dir() (string, error) {
	if d := os.Getenv("MDV_DIR"); d != "" {
		return ExpandPath(d)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "dev", "md"), nil
}

func ExpandPath(p string) (string, error) {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		p = filepath.Join(home, p[1:])
	}
	return filepath.Abs(p)
}

func ResolveFile(arg string) (string, error) {
	p, err := ExpandPath(arg)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(p)
	if err != nil {
		return "", fmt.Errorf("cannot open %s: %w", arg, err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("%s is not a regular file", arg)
	}
	if !IsMarkdown(p) {
		return "", fmt.Errorf("%s is not a Markdown file (.md / .markdown)", arg)
	}
	return p, nil
}

func IsMarkdown(p string) bool {
	ext := strings.ToLower(filepath.Ext(p))
	return ext == ".md" || ext == ".markdown"
}

type File struct {
	Path    string
	Rel     string
	ModTime time.Time
}

// MarkdownFiles walks dir recursively, skipping hidden folders.
func MarkdownFiles(dir string) []File {
	var items []File
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && p != dir && strings.HasPrefix(d.Name(), ".") {
			return filepath.SkipDir
		}
		if d.IsDir() || !IsMarkdown(p) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(dir, p)
		items = append(items, File{Path: p, Rel: rel, ModTime: info.ModTime()})
		return nil
	})
	return items
}
