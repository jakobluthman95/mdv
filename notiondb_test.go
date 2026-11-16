package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

const (
	testDBID      = "710ded4c-6e54-4339-87fa-f97ad59402e2"
	testDSID      = "de27f7ee-6715-4381-bf72-a6cf2f9f08fa"
	testOtherDSID = "25c2aab9-d524-431a-8d07-4e1b0cbd7052"
	testPageID    = "11111111-2222-3333-4444-555555555555"
	testDocID     = "99999999-8888-7777-6666-555555555555"
)

func testSchema() dataSource {
	opts := func(names ...string) *dsOptions {
		o := &dsOptions{}
		for _, n := range names {
			o.Options = append(o.Options, dsOption{Name: n})
		}
		return o
	}
	d := dataSource{
		ID: testDSID,
		Properties: map[string]dsProperty{
			"Name":                  {Type: "title"},
			"Category":              {Type: "select", Select: opts("Explanation", "Reference")},
			"Audience":              {Type: "multi_select", MultiSelect: opts("Developer", "QA")},
			"Tags":                  {Type: "multi_select", MultiSelect: opts("⚙️ Technical Documentation", "📖 Guides")},
			"Date":                  {Type: "date"},
			"Score":                 {Type: "number"},
			"Done":                  {Type: "checkbox"},
			"Created by":            {Type: "created_by"},
			"Owner Teams":           {Type: "relation"},
			"Related Documentation": {Type: "relation"},
		},
	}
	ot := d.Properties["Owner Teams"]
	ot.Relation = &struct {
		DataSourceID string `json:"data_source_id"`
		Type         string `json:"type"`
	}{testOtherDSID, "dual_property"}
	d.Properties["Owner Teams"] = ot
	rd := d.Properties["Related Documentation"]
	rd.Relation = &struct {
		DataSourceID string `json:"data_source_id"`
		Type         string `json:"type"`
	}{testDSID, "single_property"}
	d.Properties["Related Documentation"] = rd
	d.Title = append(d.Title, struct {
		PlainText string `json:"plain_text"`
	}{"Documentation"})
	return d
}

func fakeGetPage(dsID string) func(string) (notionPage, error) {
	return func(id string) (notionPage, error) {
		var p notionPage
		p.ID = id
		p.Parent.DataSourceID = dsID
		return p, nil
	}
}

func TestBuildPropsValid(t *testing.T) {
	props, summary, err := buildProps(testSchema(), []string{
		"category=explanation",
		"Audience=Developer, QA",
		"Tags=Technical Documentation",
		"Date=2026-11-16",
		"Score=3.5",
		"Done=yes",
		"Related Documentation=https://app.notion.com/p/Other-doc-" + strings.ReplaceAll(testDocID, "-", ""),
	}, fakeGetPage(testDSID))
	if err != nil {
		t.Fatal(err)
	}
	got, _ := json.Marshal(props)
	for _, want := range []string{
		`"Category":{"select":{"name":"Explanation"}}`,
		`"Audience":{"multi_select":[{"name":"Developer"},{"name":"QA"}]}`,
		`"Tags":{"multi_select":[{"name":"⚙️ Technical Documentation"}]}`,
		`"Date":{"date":{"start":"2026-11-16"}}`,
		`"Score":{"number":3.5}`,
		`"Done":{"checkbox":true}`,
		`"Related Documentation":{"relation":[{"id":"` + testDocID + `"}]}`,
	} {
		if !strings.Contains(string(got), want) {
			t.Errorf("props missing %s\ngot %s", want, got)
		}
	}
	if len(summary) != 7 {
		t.Errorf("summary = %v", summary)
	}
}

func TestBuildPropsRefusals(t *testing.T) {
	cases := map[string]string{
		"Category=Brand new":            "not an existing option",
		"Tags=Guides,Nope":              "not an existing option",
		"Owner Teams=" + testPageID:     "two-way relation",
		"Name=Something":                "use --title",
		"Created by=me":                 "can't be set",
		"Missing=x":                     "has no property",
		"Date=16/11/2026":               "not a date",
		"Score=lots":                    "not a number",
		"Done=maybe":                    "not true or false",
		"Related Documentation=garbage": "cannot find a Notion page ID",
	}
	for raw, want := range cases {
		_, _, err := buildProps(testSchema(), []string{raw}, fakeGetPage(testDSID))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want %q", raw, err, want)
		}
	}
}

func TestBuildPropsRelationWrongDatabase(t *testing.T) {
	_, _, err := buildProps(testSchema(), []string{"Related Documentation=" + testDocID}, fakeGetPage(testOtherDSID))
	if err == nil || !strings.Contains(err.Error(), "not in the database") {
		t.Fatalf("err = %v", err)
	}
	_, _, err = buildProps(testSchema(), []string{"Related Documentation=" + testDocID}, func(string) (notionPage, error) {
		return notionPage{}, errors.New("object_not_found")
	})
	if err == nil || !strings.Contains(err.Error(), "cannot read linked page") {
		t.Fatalf("err = %v", err)
	}
}

func TestBuildPropsDuplicate(t *testing.T) {
	_, _, err := buildProps(testSchema(), []string{"Category=Reference", "category=Explanation"}, fakeGetPage(testDSID))
	if err == nil || !strings.Contains(err.Error(), "more than once") {
		t.Fatalf("err = %v", err)
	}
}

// fakeNotion is a minimal Notion API that records every request.
type fakeNotion struct {
	mu       sync.Mutex
	requests []string
	created  map[string]any
	patched  []map[string]any
}

func (f *fakeNotion) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.requests = append(f.requests, r.Method+" "+r.URL.Path)
		f.mu.Unlock()
		write := func(v any) { json.NewEncoder(w).Encode(v) }
		page := map[string]any{
			"id": testPageID, "url": "https://app.notion.com/p/" + strings.ReplaceAll(testPageID, "-", ""),
			"last_edited_time": "2026-11-16T10:00:00.000Z", "last_edited_by": map[string]string{"id": "bot"},
			"parent": map[string]string{"type": "data_source_id", "data_source_id": testDSID},
		}
		switch {
		case r.Method == "GET" && r.URL.Path == "/v1/databases/"+testDBID:
			write(map[string]any{"url": "https://app.notion.com/p/db", "data_sources": []map[string]string{{"id": testDSID, "name": "Documentation"}}})
		case r.Method == "GET" && r.URL.Path == "/v1/data_sources/"+testDSID:
			s := testSchema()
			s.Parent.DatabaseID = testDBID
			write(s)
		case r.Method == "POST" && r.URL.Path == "/v1/pages":
			json.Unmarshal(body, &f.created)
			write(map[string]string{"id": testPageID})
		case r.Method == "PATCH" && r.URL.Path == "/v1/pages/"+testPageID:
			var p map[string]any
			json.Unmarshal(body, &p)
			f.patched = append(f.patched, p)
			write(page)
		case r.Method == "GET" && r.URL.Path == "/v1/pages/"+testPageID+"/markdown":
			write(map[string]string{"markdown": "body\n"})
		case r.Method == "GET" && r.URL.Path == "/v1/pages/"+testPageID:
			write(page)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			write(map[string]string{"code": "object_not_found", "message": "not found"})
		}
	}
}

func TestPushToDatabase(t *testing.T) {
	fake := &fakeNotion{}
	srv := httptest.NewServer(fake.handler(t))
	defer srv.Close()
	t.Setenv("NOTION_API_TOKEN", "secret_test")
	t.Setenv("NOTION_API_BASE_URL", srv.URL)

	dir := t.TempDir()
	doc := filepath.Join(dir, "TAP-1-test.md")
	if err := os.WriteFile(doc, []byte("# 🔍 Test doc\n\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := notionPushCmd(dir, []string{"TAP-1-test.md", "--database",
		"https://app.notion.com/p/firstvet/" + strings.ReplaceAll(testDBID, "-", "") + "?v=c4d96be8e12645768542ee082b519eee",
		"--prop", "Category=Explanation", "--prop", "Audience=Developer", "--yes"})
	if err != nil {
		t.Fatal(err)
	}

	parent, _ := json.Marshal(fake.created["parent"])
	if string(parent) != `{"data_source_id":"`+testDSID+`"}` {
		t.Errorf("parent = %s", parent)
	}
	props, _ := json.Marshal(fake.created["properties"])
	for _, want := range []string{`"Category":{"select":{"name":"Explanation"}}`, `"title":{"title":[{"text":{"content":"🔍 Test doc"},"type":"text"}]}`} {
		if !strings.Contains(string(props), want) {
			t.Errorf("created properties missing %s\ngot %s", want, props)
		}
	}

	state, err := loadNotionState(dir)
	if err != nil {
		t.Fatal(err)
	}
	e := state["TAP-1-test.md"]
	if e.PageID != testPageID || e.DataSourceID != testDSID || e.ParentID != testDBID {
		t.Fatalf("state entry = %+v", e)
	}

	// Properties-only update of the entry mdv created: the content is not replaced.
	if err := notionPushCmd(dir, []string{"TAP-1-test.md", "--prop", "Category=Reference", "--yes"}); err != nil {
		t.Fatal(err)
	}
	last, _ := json.Marshal(fake.patched[len(fake.patched)-1]["properties"])
	if !strings.Contains(string(last), `"Category":{"select":{"name":"Reference"}}`) {
		t.Errorf("update properties = %s", last)
	}

	for _, r := range fake.requests {
		method, path, _ := strings.Cut(r, " ")
		if method == "GET" {
			continue
		}
		if strings.Contains(path, "/databases") || strings.Contains(path, "/data_sources") {
			t.Errorf("wrote to a database: %s", r)
		}
		if r != "POST /v1/pages" && !strings.HasPrefix(path, "/v1/pages/"+testPageID) {
			t.Errorf("wrote outside the entry mdv created: %s", r)
		}
		if strings.HasSuffix(path, "/markdown") {
			t.Errorf("properties-only flow replaced content: %s", r)
		}
	}
}

func TestPushPropRules(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.md"), []byte("# A\n"), 0o644)
	if err := notionPushCmd(dir, []string{"a.md", "--private", "--prop", "Category=x", "--yes"}); err == nil || !strings.Contains(err.Error(), "only applies to database entries") {
		t.Errorf("err = %v", err)
	}
	if err := notionPushCmd(dir, []string{"a.md", "--private", "--database", testDBID, "--yes"}); err == nil || !strings.Contains(err.Error(), "pass one of") {
		t.Errorf("err = %v", err)
	}
	state := notionState{"a.md": {PageID: testPageID}}
	state.save(dir)
	if err := notionPushCmd(dir, []string{"a.md", "--prop", "Category=x", "--yes"}); err == nil || !strings.Contains(err.Error(), "not a database entry") {
		t.Errorf("err = %v", err)
	}
}
