package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/jakobluthman95/mdv/internal/check"
	"github.com/jakobluthman95/mdv/internal/docs"
	"github.com/jakobluthman95/mdv/internal/notion"
	"github.com/jakobluthman95/mdv/internal/safeguard"
	"github.com/jakobluthman95/mdv/internal/search"
	"github.com/jakobluthman95/mdv/internal/setup"
	"github.com/jakobluthman95/mdv/internal/share"
	"github.com/jakobluthman95/mdv/internal/skill"
	ver "github.com/jakobluthman95/mdv/internal/version"
	"github.com/jakobluthman95/mdv/internal/viewer"
)

// version is set at build time by GoReleaser and make install
// (-ldflags "-X main.version=..."). go install builds fall back to the module
// version Go records in the binary.
var version = ""

const usage = `mdv — terminal Markdown viewer

Usage:
  mdv              browse .md files in the docs folder (%s)
  mdv <file.md>    open a single file (relative or absolute path, ~ supported)
  mdv --dir        print the docs folder path
  mdv new ...      start a doc from a template (see mdv new --help)
  mdv check ...    check docs against the agent-index and template rules
  mdv find ...     find docs by agent index and text (see mdv find --help)
  mdv signals ...  look for signs that a doc's work regressed (see mdv signals --help)
  mdv impact ...   list the docs that build on a doc (see mdv impact --help)
  mdv notion ...   sync docs to Notion (API token or ntn CLI; see mdv notion --help)
  mdv share ...    share a doc by secret link (GitHub gist; see mdv share --help)
  mdv skill ...    print, diff or install the mdv-docs agent skill (see mdv skill --help)
  mdv init         set up tokens and the docs folder (~/.config/mdv/.env)
  mdv doctor       check the setup (docs folder, Notion and GitHub access)
  mdv --version    print the version
  mdv -h, --help   show this help

Set MDV_DIR to use a different docs folder. Settings (MDV_DIR, NOTION_API_TOKEN,
GITHUB_TOKEN) can also go in ~/.config/mdv/.env, or in any file named by
MDV_ENV_FILE; see .env.example.

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
	ver.Set(version)
	setup.SetWelcome(welcomeDoc)
	setup.SetEnvTemplate(envTemplate)
	skill.SetFS(skillFS)

	if err := setup.LoadEnvFile(setup.ConfigEnvFile()); err != nil {
		if setup.EnvFileExplicit() {
			fail(fmt.Errorf("MDV_ENV_FILE: %w", err))
		}
		fail(fmt.Errorf("reading %s: %w", setup.ConfigEnvFile(), err))
	}
	dir, err := docs.Dir()
	if err != nil {
		fail(err)
	}

	args := os.Args[1:]
	if len(args) > 0 && args[0] == "notion" {
		if err := notion.Run(dir, args[1:]); err != nil {
			fail(err)
		}
		return
	}
	if len(args) > 0 && args[0] == "init" {
		if err := setup.Init(args[1:]); err != nil {
			fail(err)
		}
		return
	}
	if len(args) > 0 && args[0] == "doctor" {
		if err := setup.Doctor(dir); err != nil {
			fail(err)
		}
		return
	}
	if len(args) > 0 && args[0] == "new" {
		if err := setup.New(dir, args[1:]); err != nil {
			fail(err)
		}
		return
	}
	if len(args) > 0 && args[0] == "check" {
		if err := check.Run(dir, args[1:]); err != nil {
			if errors.Is(err, check.ErrFailed) {
				os.Exit(1)
			}
			fail(err)
		}
		return
	}
	if len(args) > 0 && args[0] == "find" {
		if err := search.Run(dir, args[1:]); err != nil {
			fail(err)
		}
		return
	}
	if len(args) > 0 && args[0] == "signals" {
		if err := safeguard.RunSignals(dir, args[1:]); err != nil {
			fail(err)
		}
		return
	}
	if len(args) > 0 && args[0] == "impact" {
		if err := safeguard.RunImpact(dir, args[1:]); err != nil {
			fail(err)
		}
		return
	}
	if len(args) > 0 && args[0] == "skill" {
		if err := skill.Run(args[1:]); err != nil {
			fail(err)
		}
		return
	}
	if len(args) > 0 && args[0] == "share" {
		if err := share.Run(dir, args[1:]); err != nil {
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
		case "-v", "--version", "version":
			fmt.Println("mdv", ver.Get())
			return
		}
		if file, err = docs.ResolveFile(args[0]); err != nil {
			fail(err)
		}
	}

	if err := setup.EnsureDocsDir(dir); err != nil {
		fail(err)
	}

	if err := viewer.Run(dir, file); err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "mdv:", err)
	os.Exit(1)
}
