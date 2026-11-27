package skill

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/jakobluthman95/mdv/internal/docs"
	"github.com/jakobluthman95/mdv/internal/gitx"
	"github.com/jakobluthman95/mdv/internal/term"
	"github.com/jakobluthman95/mdv/internal/version"
)

var skillFS fs.FS

func SetFS(f fs.FS) { skillFS = f }

const skillRoot = "skills/mdv-docs"

const skillUsage = `Usage:
  mdv skill print                 print the mdv-docs agent skill bundled with this mdv
  mdv skill diff [--dir <dir>]    compare the installed skill with the bundled one (read-only)
  mdv skill install [--dir <dir>] install the bundled skill

  --dir <dir>  the skill folder (default ~/.claude/skills/mdv-docs)
  --force      replace a skill that was changed after mdv installed it
  --yes        skip the confirmation prompt

After upgrading mdv, install updates a skill it installed earlier. It never
overwrites a skill that was changed since (or that mdv didn't install) unless
--force is given; run mdv skill diff first to see what would change.`

type skillFile struct {
	name string
	data []byte
}

func bundledSkill() ([]skillFile, error) {
	var files []skillFile
	err := fs.WalkDir(skillFS, skillRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(skillFS, p)
		if err != nil {
			return err
		}
		files = append(files, skillFile{strings.TrimPrefix(p, skillRoot+"/"), b})
		return nil
	})
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	return files, err
}

func DefaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "skills", "mdv-docs")
}

type State int

const (
	Missing State = iota
	Matches
	Outdated
	Customised
)

const skillMarkerFile = ".mdv-skill.json"

type skillMarker struct {
	Version string            `json:"version"`
	Files   map[string]string `json:"files"`
}

func readSkillMarker(dir string) (skillMarker, bool) {
	var m skillMarker
	b, err := os.ReadFile(filepath.Join(dir, skillMarkerFile))
	if err != nil || json.Unmarshal(b, &m) != nil || len(m.Files) == 0 {
		return m, false
	}
	return m, true
}

func Status(dir string) (State, []string, error) {
	files, err := bundledSkill()
	if err != nil {
		return 0, nil, err
	}
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); errors.Is(err, os.ErrNotExist) {
		return Missing, nil, nil
	}
	var differing []string
	for _, f := range files {
		installed, err := os.ReadFile(filepath.Join(dir, f.name))
		if err != nil || !bytes.Equal(installed, f.data) {
			differing = append(differing, f.name)
		}
	}
	if len(differing) == 0 {
		return Matches, nil, nil
	}
	marker, ok := readSkillMarker(dir)
	if !ok {
		return Customised, differing, nil
	}
	for name, sha := range marker.Files {
		installed, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || docs.HashBytes(installed) != sha {
			return Customised, differing, nil
		}
	}
	return Outdated, differing, nil
}

func Run(args []string) error {
	if len(args) == 0 {
		fmt.Println(skillUsage)
		return nil
	}
	switch args[0] {
	case "print":
		files, err := bundledSkill()
		if err != nil {
			return err
		}
		for _, f := range files {
			if f.name == "SKILL.md" {
				fmt.Print(string(f.data))
			}
		}
		return nil
	case "diff":
		return skillDiffCmd(args[1:])
	case "install":
		return skillInstallCmd(args[1:])
	case "-h", "--help", "help":
		fmt.Println(skillUsage)
		return nil
	}
	return fmt.Errorf("unknown skill command %q (see mdv skill --help)", args[0])
}

func skillDirFlag(fs *flag.FlagSet) *string {
	return fs.String("dir", DefaultDir(), "skill folder")
}

func skillDiffCmd(args []string) error {
	fs := flag.NewFlagSet("skill diff", flag.ContinueOnError)
	dir := skillDirFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	out, state, err := skillDiff(*dir)
	if err != nil {
		return err
	}
	switch state {
	case Missing:
		fmt.Printf("No skill installed in %s (mdv skill install)\n", term.Tildify(*dir))
	case Matches:
		fmt.Printf("%s matches the skill bundled with mdv %s\n", term.Tildify(*dir), version.Get())
	case Outdated:
		fmt.Print(out)
		fmt.Printf("The installed skill is from an older mdv and unchanged since; mdv skill install updates it.\n")
	default:
		fmt.Print(out)
		fmt.Printf("The installed skill was changed after mdv installed it (or not installed by mdv); mdv skill install --force replaces it.\n")
	}
	return nil
}

func skillDiff(dir string) (string, State, error) {
	state, differing, err := Status(dir)
	if err != nil || (state != Outdated && state != Customised) {
		return "", state, err
	}
	files, err := bundledSkill()
	if err != nil {
		return "", state, err
	}
	var b strings.Builder
	for _, f := range files {
		if !slices.Contains(differing, f.name) {
			continue
		}
		installed, _ := os.ReadFile(filepath.Join(dir, f.name))
		d, err := gitx.Diff("installed", "bundled", f.name, installed, f.data)
		if err != nil {
			return "", state, err
		}
		b.WriteString(d)
	}
	return b.String(), state, nil
}

func skillInstallCmd(args []string) error {
	fs := flag.NewFlagSet("skill install", flag.ContinueOnError)
	dir := skillDirFlag(fs)
	force := fs.Bool("force", false, "install over a skill that differs")
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	if err := fs.Parse(args); err != nil {
		return err
	}
	state, differing, err := Status(*dir)
	if err != nil {
		return err
	}
	question := fmt.Sprintf("Install the mdv-docs skill (mdv %s) into %s?", version.Get(), term.Tildify(*dir))
	switch state {
	case Matches:
		if _, ok := readSkillMarker(*dir); !ok {
			if err := writeSkillMarker(*dir); err != nil {
				return err
			}
		}
		fmt.Printf("Already installed: %s matches mdv %s\n", term.Tildify(*dir), version.Get())
		return nil
	case Customised:
		if !*force {
			return fmt.Errorf("%s has a skill that was changed after mdv installed it, or that mdv didn't install (%s differs); run mdv skill diff to see how, and pass --force to replace it", term.Tildify(*dir), strings.Join(differing, ", "))
		}
		out, _, err := skillDiff(*dir)
		if err != nil {
			return err
		}
		fmt.Print(out)
		question = fmt.Sprintf("Replace the customised skill in %s with the one from mdv %s?", term.Tildify(*dir), version.Get())
	case Outdated:
		marker, _ := readSkillMarker(*dir)
		question = fmt.Sprintf("Update the mdv-docs skill in %s from mdv %s to %s (%s)?", term.Tildify(*dir), marker.Version, version.Get(), strings.Join(differing, ", "))
	}
	if err := term.Confirm(question, *yes); err != nil {
		return err
	}
	return installSkill(*dir)
}

func writeSkillMarker(dir string) error {
	files, err := bundledSkill()
	if err != nil {
		return err
	}
	m := skillMarker{Version: version.Get(), Files: map[string]string{}}
	for _, f := range files {
		m.Files[f.name] = docs.HashBytes(f.data)
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, skillMarkerFile), append(b, '\n'), 0o644)
}

func installSkill(dir string) error {
	files, err := bundledSkill()
	if err != nil {
		return err
	}
	for _, f := range files {
		p := filepath.Join(dir, f.name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, f.data, 0o644); err != nil {
			return err
		}
	}
	if err := writeSkillMarker(dir); err != nil {
		return err
	}
	fmt.Println("Installed the mdv-docs skill in", term.Tildify(dir))
	return nil
}
