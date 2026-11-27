package skill

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jakobluthman95/mdv/internal/docs"
	"github.com/jakobluthman95/mdv/internal/version"
)

func init() { SetFS(os.DirFS("../..")) }

func TestBundledSkillHasSkillMD(t *testing.T) {
	files, err := bundledSkill()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f.name == "SKILL.md" && strings.HasPrefix(string(f.data), "---\nname: mdv-docs") {
			return
		}
	}
	t.Fatalf("SKILL.md not bundled: %v", files)
}

func readSkill(t *testing.T, dir string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestSkillFreshInstallWritesMarker(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "mdv-docs")
	if state, _, err := Status(dir); err != nil || state != Missing {
		t.Fatalf("empty dir: state %v, err %v", state, err)
	}
	if err := skillInstallCmd([]string{"--dir", dir, "--yes"}); err != nil {
		t.Fatal(err)
	}
	if state, _, _ := Status(dir); state != Matches {
		t.Fatalf("after install: state %v", state)
	}
	m, ok := readSkillMarker(dir)
	if !ok || m.Version != version.Get() || m.Files["SKILL.md"] == "" || m.Files["notion-markdown.md"] == "" {
		t.Fatalf("marker: %+v, ok %v", m, ok)
	}
}

func TestSkillOutdatedUpdatesWithoutForce(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "mdv-docs")
	if err := installSkill(dir); err != nil {
		t.Fatal(err)
	}
	old := "---\nname: mdv-docs\n---\nold version\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	m, _ := readSkillMarker(dir)
	m.Version, m.Files["SKILL.md"] = "v0.2.0", docs.HashBytes([]byte(old))
	b, _ := json.Marshal(m)
	if err := os.WriteFile(filepath.Join(dir, skillMarkerFile), b, 0o644); err != nil {
		t.Fatal(err)
	}

	if state, differing, _ := Status(dir); state != Outdated || len(differing) != 1 {
		t.Fatalf("state %v, differing %v", state, differing)
	}
	if err := skillInstallCmd([]string{"--dir", dir, "--yes"}); err != nil {
		t.Fatalf("outdated skill should update without --force: %v", err)
	}
	if state, _, _ := Status(dir); state != Matches {
		t.Fatalf("after update: state %v", state)
	}
}

func TestSkillCustomisedNeedsForce(t *testing.T) {
	for name, withMarker := range map[string]bool{"edited after install": true, "not installed by mdv": false} {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "mdv-docs")
			if err := installSkill(dir); err != nil {
				t.Fatal(err)
			}
			if !withMarker {
				os.Remove(filepath.Join(dir, skillMarkerFile))
			}
			custom := "---\nname: mdv-docs\n---\ncustomised\n"
			if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(custom), 0o644); err != nil {
				t.Fatal(err)
			}
			if state, _, _ := Status(dir); state != Customised {
				t.Fatalf("state %v", state)
			}
			if diff, _, err := skillDiff(dir); err != nil || !strings.Contains(diff, "-customised") {
				t.Fatalf("diff %q, err %v", diff, err)
			}
			err := skillInstallCmd([]string{"--dir", dir, "--yes"})
			if err == nil || !strings.Contains(err.Error(), "--force") {
				t.Fatalf("should refuse without --force, got %v", err)
			}
			if readSkill(t, dir) != custom {
				t.Fatal("refused install still changed the skill")
			}
			if err := skillInstallCmd([]string{"--dir", dir, "--yes", "--force"}); err != nil {
				t.Fatal(err)
			}
			if state, _, _ := Status(dir); state != Matches {
				t.Fatalf("after --force: state %v", state)
			}
		})
	}
}

func TestSkillMatchingWithoutMarkerGetsMarker(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "mdv-docs")
	if err := installSkill(dir); err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(dir, skillMarkerFile))
	if err := skillInstallCmd([]string{"--dir", dir, "--yes"}); err != nil {
		t.Fatal(err)
	}
	if _, ok := readSkillMarker(dir); !ok {
		t.Fatal("a matching skill without a marker should get one, so later updates work")
	}
}
