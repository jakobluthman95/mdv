package check

import (
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/jakobluthman95/mdv/internal/docs"
)

var checkNow = time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC)

func issuesText(issues []Issue) string {
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
	if issues := Doc([]byte(goodSummaryDoc()), checkNow); len(issues) != 0 {
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
			issues := Doc([]byte(c.doc), checkNow)
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
	if issues := Doc([]byte(doc), checkNow); len(issues) != 0 {
		t.Fatalf("images, code blocks and code spans should not be flagged:\n%s", issuesText(issues))
	}
}

func TestTemplatesDocMatchesDocTypes(t *testing.T) {
	b, err := os.ReadFile("../../skills/mdv-docs/templates.md")
	if err != nil {
		t.Fatal(err)
	}
	doc := string(b)
	for _, dt := range docs.Types {
		start := strings.Index(doc, "## "+dt.Emoji+" `"+dt.Name+"`")
		if start < 0 {
			t.Errorf("templates.md has no section for %s", dt.Name)
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
		if strings.Join(got, "|") != strings.Join(dt.Sections, "|") {
			t.Errorf("%s: templates.md lists %v, mdv uses %v", dt.Name, got, dt.Sections)
		}
	}
}
