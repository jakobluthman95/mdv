package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const sampleIndex = "---\nticket: TAP-1\nrepo: web-monorepo\napps: [web-app]\ntype: summary\nstatus: done\nsummary: Did the thing.\nkeywords: [axios, fetch]\npr: 42\ncaveats:\n  - Watch out\nupdated: 2026-11-27\n---\n"

func TestSplitFrontmatter(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		ok      bool
		wantErr bool
		body    string
	}{
		{"none", "# Title\n\nText\n", false, false, "# Title\n\nText\n"},
		{"valid", sampleIndex + "\n# Title\n", true, false, "# Title\n"},
		{"crlf", strings.ReplaceAll(sampleIndex+"# Title\n", "\n", "\r\n"), true, false, "# Title\n"},
		{"rule not at top", "# Title\n\n---\n\nkey: value\n---\n", false, false, "# Title\n\n---\n\nkey: value\n---\n"},
		{"rule at top, prose", "---\n\nSome prose here.\n\n---\n\nMore.\n", false, false, "---\n\nSome prose here.\n\n---\n\nMore.\n"},
		{"invalid yaml", "---\nticket: [unclosed\nrepo: x\n---\n# Title\n", true, true, "# Title\n"},
		{"unclosed", "---\nticket: TAP-1\n# Title\n", false, false, "---\nticket: TAP-1\n# Title\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, body, ok, err := splitFrontmatter([]byte(c.in))
			if ok != c.ok || (err != nil) != c.wantErr || string(body) != c.body {
				t.Errorf("ok=%v err=%v body=%q", ok, err, body)
			}
		})
	}
	meta, _, _, _ := splitFrontmatter([]byte(sampleIndex))
	if meta.Ticket != "TAP-1" || meta.Apps[0] != "web-app" || meta.PR.String() != "PR #42" || meta.Caveats[0] != "Watch out" {
		t.Errorf("meta: %+v", meta)
	}
	meta, _, _, _ = splitFrontmatter([]byte("---\nrepo: x\npr: https://github.com/o/r/pull/7\n---\n"))
	if meta.PR.String() != "https://github.com/o/r/pull/7" {
		t.Errorf("pr url: %q", meta.PR)
	}
}

func TestAgentIndexDoesNotAffectSyncHash(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")
	plain := []byte("# Title\n\nText\n")
	if docHash(dir, path, plain) != hashBytes(plain) {
		t.Fatal("a doc without an agent index must hash exactly as before")
	}
	a := append([]byte(sampleIndex), plain...)
	b := append([]byte(strings.Replace(sampleIndex, "status: done", "status: in-progress", 1)), plain...)
	if docHash(dir, path, a) != hashBytes(plain) || docHash(dir, path, b) != hashBytes(plain) {
		t.Fatal("editing only the agent index must not change the sync hash")
	}
}

func TestAgentIndexStrippedForNotionAndGists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.md")
	if err := os.WriteFile(path, append([]byte(sampleIndex), "# 🔍 Title\n\nBody\n"...), 0o644); err != nil {
		t.Fatal(err)
	}
	content, err := readDocBody(path)
	if err != nil {
		t.Fatal(err)
	}
	icon, title, body := splitTitle(content)
	if icon != "🔍" || title != "Title" || strings.Contains(string(body), "ticket:") {
		t.Errorf("icon %q title %q body %q", icon, title, body)
	}
	if strings.Contains(string(nfmForGist(content)), "ticket:") {
		t.Error("gist copy contains the agent index")
	}
}

func TestViewerShowsAgentIndexHeader(t *testing.T) {
	out, err := renderForTerminal(sampleIndex+"# Title\n\nBody\n", "dark", 80)
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
