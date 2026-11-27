package notion

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jakobluthman95/mdv/internal/docs"
)

func TestDescribeSchema(t *testing.T) {
	info := describeSchema(testSchema())
	if info.DataSourceID != testDSID {
		t.Fatalf("data source = %q", info.DataSourceID)
	}
	if info.Properties[0].Type != "title" {
		t.Fatalf("title should come first, got %+v", info.Properties[0])
	}
	byName := map[string]schemaProperty{}
	for _, p := range info.Properties {
		byName[p.Name] = p
	}
	cases := map[string]string{
		"Name":                  "title",
		"Category":              "yes",
		"Tags":                  "yes",
		"Date":                  "yes",
		"Created by":            "no",
		"Owner Teams":           "no",
		"Related Documentation": "yes",
	}
	for name, want := range cases {
		if got := byName[name].Settable; got != want {
			t.Errorf("%s settable = %q, want %q", name, got, want)
		}
	}
	if got := strings.Join(byName["Tags"].Options, "|"); got != "⚙️ Technical Documentation|📖 Guides" {
		t.Errorf("Tags options = %q", got)
	}
	if !strings.Contains(byName["Owner Teams"].Note, "two-way") {
		t.Errorf("Owner Teams note = %q", byName["Owner Teams"].Note)
	}
	out := info.String()
	for _, want := range []string{"✓ Category (select)", "options: Explanation, Reference", "✗ Owner Teams (relation)"} {
		if !strings.Contains(out, want) {
			t.Errorf("output misses %q:\n%s", want, out)
		}
	}
}

func TestDescribePage(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "doc.md"), []byte("# Doc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var p notionPage
	p.ID, p.URL, p.Parent.Type, p.Parent.PageID = testPageID, "https://notion.so/x", "page_id", testDocID
	state := State{"doc.md": {PageID: testPageID, SHA256: docs.HashBytes([]byte("# Doc\n"))}}

	info := describePage(dir, p, state)
	if !info.CreatedByMdv || info.Doc != "doc.md" || info.ParentID != testDocID || info.SyncState != "✓ synced" {
		t.Fatalf("got %+v", info)
	}
	if !strings.Contains(info.String(), "created by   mdv, from doc.md") {
		t.Errorf("output: %s", info.String())
	}

	other := describePage(dir, p, State{})
	if other.CreatedByMdv || !strings.Contains(other.String(), "not mdv") {
		t.Errorf("unowned page: %+v", other)
	}
}
