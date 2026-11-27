package setup

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jakobluthman95/mdv/internal/skill"
)

func fakeTool(t *testing.T, dir, name, script string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func runDoctor(t *testing.T, tools map[string]string) string {
	t.Helper()
	home, bin, docsDir := t.TempDir(), t.TempDir(), t.TempDir()
	for name, script := range tools {
		fakeTool(t, bin, name, script)
	}
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("MDV_ENV_FILE", "")
	t.Setenv("NOTION_API_TOKEN", "")
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("PATH", bin)
	skill.SetFS(fstest.MapFS{})

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err := Doctor(docsDir)
	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

func wantLines(t *testing.T, out string, want ...string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(out, w) {
			t.Errorf("missing %q in:\n%s", w, out)
		}
	}
}

func TestDoctorNotSetUpIsNotAProblem(t *testing.T) {
	out := runDoctor(t, nil)
	wantLines(t, out,
		"· notion   not set up (optional, only for mdv notion)",
		"· github   not set up (optional, only for mdv share)",
		"· git      not found (needed for mdv share, notion diff and skill diff)")
	if strings.Contains(out, "✗") || strings.Contains(out, "Fix the") {
		t.Errorf("nothing is broken:\n%s", out)
	}
}

func TestDoctorWorkingCLIs(t *testing.T) {
	out := runDoctor(t, map[string]string{
		"ntn": "exit 0",
		"gh":  "echo \"Token scopes: 'gist', 'repo'\"",
		"git": "exit 0",
	})
	wantLines(t, out, "✓ notion   ntn CLI logged in", "✓ github   gh CLI logged in with the gist scope", "✓ git      found")
	if strings.Contains(out, "✗") {
		t.Errorf("nothing is broken:\n%s", out)
	}
}

func TestDoctorBrokenCLIs(t *testing.T) {
	out := runDoctor(t, map[string]string{
		"ntn": "exit 1",
		"gh":  "echo \"Token scopes: 'repo'\"",
	})
	wantLines(t, out,
		"✗ notion   ntn is installed but not logged in",
		"✗ github   gh is logged in without the gist scope",
		"✗ git      git not found",
		"Fix the ✗ items above (notion, git, github); Notion and GitHub are optional.")
	out = runDoctor(t, map[string]string{"gh": "exit 1"})
	wantLines(t, out, "· notion   not set up", "✗ github   gh is installed but not logged in")
}
