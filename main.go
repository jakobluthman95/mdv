package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const usage = `mdv — terminal Markdown viewer

Usage:
  mdv              browse .md files in the docs folder (%s)
  mdv <file.md>    open a single file (relative or absolute path, ~ supported)
  mdv --dir        print the docs folder path
  mdv notion ...   sync docs to Notion (API token or ntn CLI; see mdv notion --help)
  mdv share ...    share a doc by secret link (GitHub gist; see mdv share --help)
  mdv doctor       check the setup (docs folder, Notion and GitHub access)
  mdv -h, --help   show this help

Set MDV_DIR to use a different docs folder. Settings (MDV_DIR, NOTION_API_TOKEN,
GITHUB_TOKEN) can also go in ~/.config/mdv/.env; see .env.example.

Keys:
  ↑/↓ j/k, mouse wheel   scroll / move
  PgUp/PgDn, space, b    page
  u/d                    half page
  g/G                    top / bottom
  /                      filter (list)
  enter                  open file
  r                      reload file
  esc, backspace         back to list
  q, ctrl+c              quit
`

func main() {
	if err := loadEnvFile(configEnvFile()); err != nil {
		fail(fmt.Errorf("reading %s: %w", configEnvFile(), err))
	}
	dir, err := docsDir()
	if err != nil {
		fail(err)
	}

	args := os.Args[1:]
	if len(args) > 0 && args[0] == "notion" {
		if err := runNotion(dir, args[1:]); err != nil {
			fail(err)
		}
		return
	}
	if len(args) > 0 && args[0] == "doctor" {
		if err := doctorCmd(dir); err != nil {
			fail(err)
		}
		return
	}
	if len(args) > 0 && args[0] == "share" {
		if err := runShare(dir, args[1:]); err != nil {
			fail(err)
		}
		return
	}
	if len(args) > 1 {
		fail(errors.New("expected at most one argument (see mdv --help)"))
	}

	var file string
	if len(args) == 1 {
		switch args[0] {
		case "-h", "--help":
			fmt.Printf(usage, dir)
			return
		case "--dir":
			fmt.Println(dir)
			return
		}
		if file, err = resolveFile(args[0]); err != nil {
			fail(err)
		}
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		fail(err)
	}

	style := "light"
	if lipgloss.HasDarkBackground() {
		style = "dark"
	}

	m := newModel(dir, file, style)
	if _, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run(); err != nil {
		fail(err)
	}
}

func docsDir() (string, error) {
	if d := os.Getenv("MDV_DIR"); d != "" {
		return expandPath(d)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "dev", "md"), nil
}

func expandPath(p string) (string, error) {
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		p = filepath.Join(home, p[1:])
	}
	return filepath.Abs(p)
}

func resolveFile(arg string) (string, error) {
	p, err := expandPath(arg)
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
	if !isMarkdown(p) {
		return "", fmt.Errorf("%s is not a Markdown file (.md / .markdown)", arg)
	}
	return p, nil
}

func isMarkdown(p string) bool {
	ext := strings.ToLower(filepath.Ext(p))
	return ext == ".md" || ext == ".markdown"
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "mdv:", err)
	os.Exit(1)
}
