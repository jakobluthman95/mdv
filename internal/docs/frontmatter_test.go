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

func TestSafeguardFields(t *testing.T) {
	in := "---\nticket: TAP-2\nrepo: web-monorepo\nstatus: regressed\nmerged: 2026-11-01\nbuilds_on: [TAP-1, web-monorepo/TAP-0-base.md]\nverification:\n  - Logged in and checked the header\nissues:\n  - ticket: TAP-555\n    found: 2026-11-20\n    note: Header dropped again\n  - ticket: TAP-556\n    found: 2026-11-21\n    fixed_by: TAP-557\n---\n# T\n"
	meta, _, ok, err := SplitFrontmatter([]byte(in))
	if !ok || err != nil {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if meta.Merged != "2026-11-01" || len(meta.BuildsOn) != 2 || meta.BuildsOn[1] != "web-monorepo/TAP-0-base.md" || len(meta.Verification) != 1 {
		t.Fatalf("meta: %+v", meta)
	}
	if len(meta.Issues) != 2 || meta.Issues[0] != (Issue{Ticket: "TAP-555", Found: "2026-11-20", Note: "Header dropped again"}) || meta.Issues[1].FixedBy != "TAP-557" {
		t.Fatalf("issues: %+v", meta.Issues)
	}
	if open := meta.OpenIssues(); len(open) != 1 || open[0].Ticket != "TAP-555" {
		t.Fatalf("open issues: %+v", open)
	}
	if w := meta.RegressionWarning(); w != "regressed (TAP-555)" {
		t.Fatalf("warning: %q", w)
	}
	meta.Status = "done"
	if w := meta.RegressionWarning(); w != "open issues (TAP-555)" {
		t.Fatalf("warning: %q", w)
	}
}
