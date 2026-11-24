package main

import (
	"bytes"
	"embed"
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
)

//go:embed skills/mdv-docs
var skillFS embed.FS

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
		b, err := skillFS.ReadFile(p)
		if err != nil {
			return err
		}
		files = append(files, skillFile{strings.TrimPrefix(p, skillRoot+"/"), b})
		return nil
	})
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	return files, err
}

func defaultSkillDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "skills", "mdv-docs")
}

type skillState int

const (
	skillMissing skillState = iota
	skillMatches
	skillOutdated
	skillCustomised
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

func skillStatus(dir string) (skillState, []string, error) {
	files, err := bundledSkill()
	if err != nil {
		return 0, nil, err
	}
	if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); errors.Is(err, os.ErrNotExist) {
		return skillMissing, nil, nil
	}
	var differing []string
	for _, f := range files {
		installed, err := os.ReadFile(filepath.Join(dir, f.name))
		if err != nil || !bytes.Equal(installed, f.data) {
			differing = append(differing, f.name)
		}
	}
	if len(differing) == 0 {
		return skillMatches, nil, nil
	}
	marker, ok := readSkillMarker(dir)
	if !ok {
		return skillCustomised, differing, nil
	}
	for name, sha := range marker.Files {
		installed, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || hashBytes(installed) != sha {
			return skillCustomised, differing, nil
		}
	}
	return skillOutdated, differing, nil
}

func runSkill(args []string) error {
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
	return fs.String("dir", defaultSkillDir(), "skill folder")
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
	case skillMissing:
		fmt.Printf("No skill installed in %s (mdv skill install)\n", tildify(*dir))
	case skillMatches:
		fmt.Printf("%s matches the skill bundled with mdv %s\n", tildify(*dir), mdvVersion())
	case skillOutdated:
		fmt.Print(out)
		fmt.Printf("The installed skill is from an older mdv and unchanged since; mdv skill install updates it.\n")
	default:
		fmt.Print(out)
		fmt.Printf("The installed skill was changed after mdv installed it (or not installed by mdv); mdv skill install --force replaces it.\n")
	}
	return nil
}

func skillDiff(dir string) (string, skillState, error) {
	state, differing, err := skillStatus(dir)
	if err != nil || (state != skillOutdated && state != skillCustomised) {
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
		d, err := gitDiff("installed", "bundled", f.name, installed, f.data)
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
	state, differing, err := skillStatus(*dir)
	if err != nil {
		return err
	}
	question := fmt.Sprintf("Install the mdv-docs skill (mdv %s) into %s?", mdvVersion(), tildify(*dir))
	switch state {
	case skillMatches:
		if _, ok := readSkillMarker(*dir); !ok {
			if err := writeSkillMarker(*dir); err != nil {
				return err
			}
		}
		fmt.Printf("Already installed: %s matches mdv %s\n", tildify(*dir), mdvVersion())
		return nil
	case skillCustomised:
		if !*force {
			return fmt.Errorf("%s has a skill that was changed after mdv installed it, or that mdv didn't install (%s differs); run mdv skill diff to see how, and pass --force to replace it", tildify(*dir), strings.Join(differing, ", "))
		}
		out, _, err := skillDiff(*dir)
		if err != nil {
			return err
		}
		fmt.Print(out)
		question = fmt.Sprintf("Replace the customised skill in %s with the one from mdv %s?", tildify(*dir), mdvVersion())
	case skillOutdated:
		marker, _ := readSkillMarker(*dir)
		question = fmt.Sprintf("Update the mdv-docs skill in %s from mdv %s to %s (%s)?", tildify(*dir), marker.Version, mdvVersion(), strings.Join(differing, ", "))
	}
	if err := confirm(question, *yes); err != nil {
		return err
	}
	return installSkill(*dir)
}

func writeSkillMarker(dir string) error {
	files, err := bundledSkill()
	if err != nil {
		return err
	}
	m := skillMarker{Version: mdvVersion(), Files: map[string]string{}}
	for _, f := range files {
		m.Files[f.name] = hashBytes(f.data)
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
	fmt.Println("Installed the mdv-docs skill in", tildify(dir))
	return nil
}
