package setup

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/jakobluthman95/mdv/internal/docs"
	"github.com/jakobluthman95/mdv/internal/notion"
	"github.com/jakobluthman95/mdv/internal/share"
	"github.com/jakobluthman95/mdv/internal/skill"
	"github.com/jakobluthman95/mdv/internal/term"
)

// Doctor checks the setup without changing anything.
func Doctor(dir string) error {
	ok := func(label, detail string) { fmt.Printf("✓ %-8s %s\n", label, detail) }
	bad := func(label, detail string) { fmt.Printf("✗ %-8s %s\n", label, detail) }
	info := func(label, detail string) { fmt.Printf("· %-8s %s\n", label, detail) }
	var problems []string
	report := func(label string, good bool, detail string) {
		if good {
			ok(label, detail)
		} else {
			bad(label, detail)
			problems = append(problems, label)
		}
	}
	optional := func(label string, state term.Access, detail string) {
		if state == term.NotSetUp {
			info(label, detail)
		} else {
			report(label, state == term.AccessOK, detail)
		}
	}

	if _, err := os.Stat(ConfigEnvFile()); err == nil {
		detail := term.Tildify(ConfigEnvFile())
		if EnvFileExplicit() {
			detail += " (from MDV_ENV_FILE)"
		}
		ok("config", detail)
	} else {
		info("config", term.Tildify(ConfigEnvFile())+" not found (optional; run mdv init to create it)")
	}

	if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
		ok("docs", fmt.Sprintf("%s (%d docs)", term.Tildify(dir), len(docs.MarkdownFiles(dir))))
	} else {
		report("docs", false, term.Tildify(dir)+" does not exist yet (mdv creates it on first run)")
	}

	state, detail := notion.CheckAccess()
	optional("notion", state, detail)

	gh, ghDetail := share.CheckAccess()
	switch _, err := exec.LookPath("git"); {
	case err == nil:
		ok("git", "found")
	case gh != term.NotSetUp:
		report("git", false, "git not found; mdv share needs it to upload images")
	default:
		info("git", "not found (needed for mdv share, notion diff and skill diff)")
	}
	optional("github", gh, ghDetail)

	switch state, _, err := skill.Status(skill.DefaultDir()); {
	case err != nil:
		info("skill", err.Error())
	case state == skill.Matches:
		ok("skill", term.Tildify(skill.DefaultDir())+" matches this mdv version")
	case state == skill.Outdated:
		info("skill", term.Tildify(skill.DefaultDir())+" is from an older mdv: run mdv skill install to update it")
	case state == skill.Customised:
		info("skill", term.Tildify(skill.DefaultDir())+" is customised (changed after install, or not installed by mdv): mdv skill diff")
	default:
		info("skill", "not installed in "+term.Tildify(skill.DefaultDir())+": mdv skill install")
	}

	if len(problems) > 0 {
		fmt.Println(term.DimStyle.Render("\nFix the ✗ items above (" + strings.Join(problems, ", ") + "); Notion and GitHub are optional."))
	}
	return nil
}
