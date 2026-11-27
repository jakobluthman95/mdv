package docs

import (
	"path/filepath"
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
			_, body, ok, err := SplitFrontmatter([]byte(c.in))
			if ok != c.ok || (err != nil) != c.wantErr || string(body) != c.body {
				t.Errorf("ok=%v err=%v body=%q", ok, err, body)
			}
		})
	}
	meta, _, _, _ := SplitFrontmatter([]byte(sampleIndex))
	if meta.Ticket != "TAP-1" || meta.Apps[0] != "web-app" || meta.PR.String() != "PR #42" || meta.Caveats[0] != "Watch out" {
		t.Errorf("meta: %+v", meta)
	}
	meta, _, _, _ = SplitFrontmatter([]byte("---\nrepo: x\npr: https://github.com/o/r/pull/7\n---\n"))
	if meta.PR.String() != "https://github.com/o/r/pull/7" {
		t.Errorf("pr url: %q", meta.PR)
	}
}

func TestAgentIndexDoesNotAffectSyncHash(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.md")
	plain := []byte("# Title\n\nText\n")
	if Hash(dir, path, plain) != HashBytes(plain) {
		t.Fatal("a doc without an agent index must hash exactly as before")
	}
	a := append([]byte(sampleIndex), plain...)
	b := append([]byte(strings.Replace(sampleIndex, "status: done", "status: in-progress", 1)), plain...)
	if Hash(dir, path, a) != HashBytes(plain) || Hash(dir, path, b) != HashBytes(plain) {
		t.Fatal("editing only the agent index must not change the sync hash")
	}
}
