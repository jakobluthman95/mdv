package search

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeDocs(t *testing.T, docs map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range docs {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func findTestDocs(t *testing.T) string {
	return writeDocs(t, map[string]string{
		"web-monorepo/TAP-222-remove-axios.md": "---\nticket: TAP-222\nrepo: web-monorepo\napps: [web-app]\ntype: summary\nstatus: done\nsummary: Replaced axios with Nuxt $fetch.\nkeywords: [axios, fetch, ofetch]\npaths: [packages/http]\npr: 1787\ncaveats: [Interceptors moved to ofetch hooks]\nupdated: 2026-11-20\n---\n# Remove axios\n\nBody.\n",
		"web-monorepo/TAP-333-fetch-bug.md":    "---\nticket: TAP-333\nrepo: web-monorepo\ntype: bug\nstatus: in-progress\nsummary: $fetch drops headers.\nkeywords: [fetch, headers]\nrelated: [TAP-222]\nupdated: 2026-11-27\n---\n# Fetch bug\n",
		"web-monorepo/web-app/TAP-444-old.md":  "# Old doc about fetch\n\nWe call fetch here and fetch there.\n",
		"firstvet-api/notes.md":                "# API notes\n\nNothing about http clients.\n",
	})
}

func TestFindRanking(t *testing.T) {
	dir := findTestDocs(t)
	got := findDocs(dir, findQuery{terms: []string{"axios"}}, 0)
	if len(got) != 1 || got[0].Meta.Ticket != "TAP-222" {
		t.Fatalf("axios: %+v", got)
	}
	got = findDocs(dir, findQuery{terms: []string{"fetch"}}, 0)
	if len(got) != 3 || !got[0].Indexed || got[2].Indexed {
		t.Fatalf("keyword matches should rank above text-only matches: %+v", got)
	}
	got = findDocs(dir, findQuery{terms: []string{"TAP-222"}}, 0)
	if len(got) != 2 || got[0].Meta.Ticket != "TAP-222" || got[1].Meta.Ticket != "TAP-333" {
		t.Fatalf("ticket first, then docs that list it as related: %+v", got)
	}
}

func TestFindFiltersAndInference(t *testing.T) {
	dir := findTestDocs(t)
	got := findDocs(dir, findQuery{app: "web-app"}, 0)
	if len(got) != 2 {
		t.Fatalf("app filter (indexed + inferred from folder): %+v", got)
	}
	got = findDocs(dir, findQuery{ticket: "TAP-444"}, 0)
	if len(got) != 1 || got[0].Indexed || got[0].Meta.Repo != "web-monorepo" {
		t.Fatalf("ticket inferred from file name: %+v", got)
	}
	got = findDocs(dir, findQuery{status: "in-progress"}, 0)
	if len(got) != 1 || got[0].Meta.Ticket != "TAP-333" {
		t.Fatalf("status filter: %+v", got)
	}
	got = findDocs(dir, findQuery{pp: "packages/http/src"}, 0)
	if len(got) != 1 || got[0].Meta.Ticket != "TAP-222" {
		t.Fatalf("path overlap: %+v", got)
	}
	got = findDocs(dir, findQuery{repo: "firstvet-api", terms: []string{"fetch"}}, 0)
	if len(got) != 0 {
		t.Fatalf("repo filter: %+v", got)
	}
}

func captureStdout(t *testing.T, fn func() error) string {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err := fn()
	w.Close()
	os.Stdout = old
	b, _ := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestFindJSONAndUnindexed(t *testing.T) {
	dir := findTestDocs(t)
	out := captureStdout(t, func() error { return Run(dir, []string{"axios", "--json"}) })
	var results []findResult
	if err := json.Unmarshal([]byte(out), &results); err != nil || len(results) != 1 {
		t.Fatalf("json: %v\n%s", err, out)
	}
	r := results[0]
	if r.Path != "web-monorepo/TAP-222-remove-axios.md" || !r.Indexed || r.Meta.PR != "1787" || len(r.Meta.Caveats) != 1 || len(r.Matched) == 0 {
		t.Fatalf("result: %+v", r)
	}
	out = captureStdout(t, func() error { return Run(dir, []string{"--unindexed", "--json"}) })
	var paths []string
	if err := json.Unmarshal([]byte(out), &paths); err != nil || len(paths) != 2 {
		t.Fatalf("unindexed: %v\n%s", err, out)
	}
	out = captureStdout(t, func() error { return Run(dir, []string{"nomatch", "--json"}) })
	if out != "[]\n" {
		t.Fatalf("no results should be an empty JSON array, got %q", out)
	}
}

func TestFindRegressionWarnings(t *testing.T) {
	dir := writeDocs(t, map[string]string{
		"web-monorepo/TAP-222-remove-axios.md": "---\nticket: TAP-222\nrepo: web-monorepo\nstatus: regressed\nkeywords: [axios]\nissues:\n  - ticket: TAP-555\n    found: 2026-11-20\n---\n# Remove axios\n",
		"web-monorepo/TAP-333-retries.md":      "---\nticket: TAP-333\nrepo: web-monorepo\nstatus: done\nkeywords: [axios, retries]\nbuilds_on: [TAP-222]\n---\n# Retries\n",
		"web-monorepo/TAP-444-unrelated.md":    "---\nticket: TAP-444\nrepo: web-monorepo\nstatus: done\nkeywords: [axios, cache]\n---\n# Cache\n",
	})
	out := captureStdout(t, func() error { return Run(dir, []string{"axios", "--json"}) })
	var results []findResult
	if err := json.Unmarshal([]byte(out), &results); err != nil || len(results) != 3 {
		t.Fatalf("json: %v\n%s", err, out)
	}
	got := map[string][]string{}
	for _, r := range results {
		got[r.Meta.Ticket] = r.Warnings
	}
	if w := got["TAP-222"]; len(w) != 1 || w[0] != "regressed (TAP-555)" {
		t.Errorf("regressed doc: %q", w)
	}
	if w := got["TAP-333"]; len(w) != 1 || w[0] != "builds on TAP-222, which regressed (TAP-555)" {
		t.Errorf("doc building on it: %q", w)
	}
	if w := got["TAP-444"]; w == nil || len(w) != 0 {
		t.Errorf("unrelated doc should have an empty list: %#v", w)
	}
	text := captureStdout(t, func() error { return Run(dir, []string{"axios"}) })
	if !strings.Contains(text, "⚠ regressed (TAP-555)") || !strings.Contains(text, "⚠ builds on TAP-222, which regressed (TAP-555)") {
		t.Errorf("text output:\n%s", text)
	}
}
