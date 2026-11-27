package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

var checkNow = time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC)

func issuesText(issues []checkIssue) string {
	var b strings.Builder
	for _, i := range issues {
		b.WriteString(i.Level + ": " + i.Message + "\n")
	}
	return b.String()
}

func goodSummaryDoc() string {
	return "---\nticket: TAP-1\nrepo: web-monorepo\ntype: summary\nstatus: done\nsummary: Replaced axios with Nuxt $fetch in web-app and moved interceptors to hooks.\nkeywords: [axios, fetch]\nupdated: 2026-11-27\n---\n# ✅ TAP-1: Remove axios\n\n## What changed\n\n`packages/http/src/client.ts` uses `$fetch.create`.\n\n## Why\n\nSmaller bundle.\n\n## How to verify\n\nRun the app.\n\n## Caveats\n\nNone.\n\n## Follow-ups\n\nNone.\n\n## For future work\n\nKeep hooks in one place.\n"
}

func TestCheckCleanDoc(t *testing.T) {
	if issues := checkDoc([]byte(goodSummaryDoc()), checkNow); len(issues) != 0 {
		t.Fatalf("expected no issues:\n%s", issuesText(issues))
	}
}

func TestCheckFindsProblems(t *testing.T) {
	cases := []struct {
		name, doc, want, level string
	}{
		{"no index", "# Title\n", "no agent index", "error"},
		{"invalid yaml", "---\nrepo: [x\n---\n# T\n", "not valid YAML", "error"},
		{"missing field", strings.Replace(goodSummaryDoc(), "keywords: [axios, fetch]\n", "", 1), "keywords is required", "error"},
		{"empty summary", strings.Replace(goodSummaryDoc(), "summary: Replaced axios with Nuxt $fetch in web-app and moved interceptors to hooks.", `summary: ""`, 1), "summary is required", "error"},
		{"unknown type", strings.Replace(goodSummaryDoc(), "type: summary", "type: notes", 1), "unknown type", "error"},
		{"unknown status", strings.Replace(goodSummaryDoc(), "status: done", "status: finished", 1), "unknown status", "error"},
		{"missing section", strings.Replace(goodSummaryDoc(), "## How to verify\n\nRun the app.\n\n", "", 1), `missing section "## How to verify"`, "error"},
		{"empty section in done doc", strings.Replace(goodSummaryDoc(), "Smaller bundle.\n", "", 1), `"## Why" is empty`, "warning"},
		{"relative link", strings.Replace(goodSummaryDoc(), "Smaller bundle.", "See [the plan](other-doc.md).", 1), "relative link", "warning"},
		{"momentary wording", strings.Replace(goodSummaryDoc(), "Smaller bundle.", "As discussed yesterday.", 1), "only makes sense in the moment", "warning"},
		{"thin summary", strings.Replace(goodSummaryDoc(), "summary: Replaced axios with Nuxt $fetch in web-app and moved interceptors to hooks.", "summary: Remove axios", 1), "summary is too thin", "warning"},
		{"stale in-progress", strings.Replace(strings.Replace(goodSummaryDoc(), "status: done", "status: in-progress", 1), "updated: 2026-11-27", "updated: 2026-09-01", 1), "not updated since", "warning"},
		{"superseded without pointer", strings.Replace(goodSummaryDoc(), "status: done", "status: superseded", 1), "superseded_by is empty", "warning"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			issues := checkDoc([]byte(c.doc), checkNow)
			for _, i := range issues {
				if strings.Contains(i.Message, c.want) && i.Level == c.level {
					return
				}
			}
			t.Errorf("want %s containing %q, got:\n%s", c.level, c.want, issuesText(issues))
		})
	}
}

func TestCheckIgnoresCodeAndImages(t *testing.T) {
	doc := strings.Replace(goodSummaryDoc(), "Smaller bundle.", "Smaller bundle.\n\n![shot](doc/shot.png)\n\n```md\n[x](relative.md) as discussed yesterday\n```\n\n`see above`", 1)
	if issues := checkDoc([]byte(doc), checkNow); len(issues) != 0 {
		t.Fatalf("images, code blocks and code spans should not be flagged:\n%s", issuesText(issues))
	}
}

func TestNewDocMatchesTemplateAndFailsCheckUntilFilled(t *testing.T) {
	for _, dt := range docTypes {
		doc := newDocContent(dt, "TAP-9", "web-monorepo", "web-app", "tap-9-x", "Thing", "2026-11-27")
		meta, body, ok, err := splitFrontmatter([]byte(doc))
		if !ok || err != nil || meta.Type != dt.name || meta.Status != "in-progress" || meta.Apps[0] != "web-app" || meta.Branch != "tap-9-x" {
			t.Fatalf("%s: index %+v ok=%v err=%v", dt.name, meta, ok, err)
		}
		if !strings.HasPrefix(string(body), "# "+dt.emoji+" TAP-9: Thing\n") {
			t.Errorf("%s: H1 %q", dt.name, strings.SplitN(string(body), "\n", 2)[0])
		}
		text := issuesText(checkDoc([]byte(doc), checkNow))
		if strings.Contains(text, "missing section") || !strings.Contains(text, "summary is required") || !strings.Contains(text, "keywords is required") {
			t.Errorf("%s: a fresh doc has every section and fails only on what must be filled in:\n%s", dt.name, text)
		}
	}
}

func TestNewCmdWritesOnceAndRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	args := []string{"bug", "TAP-7", "Header Bug!", "--repo", "web-monorepo", "--app", "web-app"}
	if err := newCmd(dir, args); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "web-monorepo", "web-app", "TAP-7-header-bug.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if err := newCmd(dir, args); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("second new should refuse, got %v", err)
	}
	if err := newCmd(dir, []string{"notes", "x", "--repo", "r"}); err == nil || !strings.Contains(err.Error(), "unknown type") {
		t.Fatalf("unknown type: %v", err)
	}
}

func TestTemplatesDocMatchesDocTypes(t *testing.T) {
	b, err := os.ReadFile("skills/mdv-docs/templates.md")
	if err != nil {
		t.Fatal(err)
	}
	doc := string(b)
	for _, dt := range docTypes {
		start := strings.Index(doc, "## "+dt.emoji+" `"+dt.name+"`")
		if start < 0 {
			t.Errorf("templates.md has no section for %s", dt.name)
			continue
		}
		rest := doc[start+1:]
		if end := strings.Index(rest, "\n## "); end >= 0 {
			rest = rest[:end]
		}
		var got []string
		for _, m := range regexp.MustCompile("(?m)^\\| \\*\\*([^*]+)\\*\\* \\|").FindAllStringSubmatch(rest, -1) {
			got = append(got, m[1])
		}
		if strings.Join(got, "|") != strings.Join(dt.sections, "|") {
			t.Errorf("%s: templates.md lists %v, mdv uses %v", dt.name, got, dt.sections)
		}
	}
}
