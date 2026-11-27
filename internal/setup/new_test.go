package setup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jakobluthman95/mdv/internal/check"
	"github.com/jakobluthman95/mdv/internal/docs"
)

var checkNow = time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC)

func issuesText(issues []check.Issue) string {
	var b strings.Builder
	for _, i := range issues {
		b.WriteString(i.Level + ": " + i.Message + "\n")
	}
	return b.String()
}

func TestNewDocMatchesTemplateAndFailsCheckUntilFilled(t *testing.T) {
	for _, dt := range docs.Types {
		doc := newDocContent(dt, "TAP-9", "web-monorepo", "web-app", "tap-9-x", "Thing", "2026-11-27")
		meta, body, ok, err := docs.SplitFrontmatter([]byte(doc))
		if !ok || err != nil || meta.Type != dt.Name || meta.Status != "in-progress" || meta.Apps[0] != "web-app" || meta.Branch != "tap-9-x" {
			t.Fatalf("%s: index %+v ok=%v err=%v", dt.Name, meta, ok, err)
		}
		if !strings.HasPrefix(string(body), "# "+dt.Emoji+" TAP-9: Thing\n") {
			t.Errorf("%s: H1 %q", dt.Name, strings.SplitN(string(body), "\n", 2)[0])
		}
		text := issuesText(check.Doc([]byte(doc), checkNow))
		if strings.Contains(text, "missing section") || !strings.Contains(text, "summary is required") || !strings.Contains(text, "keywords is required") {
			t.Errorf("%s: a fresh doc has every section and fails only on what must be filled in:\n%s", dt.Name, text)
		}
	}
}

func TestNewCmdWritesOnceAndRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	args := []string{"bug", "TAP-7", "Header Bug!", "--repo", "web-monorepo", "--app", "web-app"}
	if err := New(dir, args); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "web-monorepo", "web-app", "TAP-7-header-bug.md")
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if err := New(dir, args); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("second new should refuse, got %v", err)
	}
	if err := New(dir, []string{"notes", "x", "--repo", "r"}); err == nil || !strings.Contains(err.Error(), "unknown type") {
		t.Fatalf("unknown type: %v", err)
	}
}
