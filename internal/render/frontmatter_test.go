package render

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/jakobluthman95/mdv/internal/docs"
)

const sampleIndex = "---\nticket: TAP-1\nrepo: web-monorepo\napps: [web-app]\ntype: summary\nstatus: done\nsummary: Did the thing.\nkeywords: [axios, fetch]\npr: 42\ncaveats:\n  - Watch out\nupdated: 2026-11-27\n---\n"

func TestAgentIndexStrippedForNotionAndGists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.md")
	if err := os.WriteFile(path, append([]byte(sampleIndex), "# 🔍 Title\n\nBody\n"...), 0o644); err != nil {
		t.Fatal(err)
	}
	content, err := docs.ReadDocBody(path)
	if err != nil {
		t.Fatal(err)
	}
	icon, title, body := docs.SplitTitle(content)
	if icon != "🔍" || title != "Title" || strings.Contains(string(body), "ticket:") {
		t.Errorf("icon %q title %q body %q", icon, title, body)
	}
	if strings.Contains(string(ForGist(content)), "ticket:") {
		t.Error("gist copy contains the agent index")
	}
}

func TestViewerShowsAgentIndexHeader(t *testing.T) {
	out, err := ForTerminal(sampleIndex+"# Title\n\nBody\n", "dark", 80)
	if err != nil {
		t.Fatal(err)
	}
	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(out, "")
	for _, want := range []string{"🤖 TAP-1 · web-monorepo/web-app · summary · done · PR #42", "Did the thing.", "keywords: axios, fetch", "⚠ Watch out"} {
		if !strings.Contains(plain, want) {
			t.Errorf("missing %q in:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "ticket:") || strings.Index(plain, "🤖") > strings.Index(plain, "Title") {
		t.Errorf("raw YAML shown, or header not above the doc:\n%s", plain)
	}
}
