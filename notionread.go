package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

type schemaProperty struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Options  []string `json:"options,omitempty"`
	Settable string   `json:"settable"`
	Note     string   `json:"note,omitempty"`
}

type schemaInfo struct {
	Name         string           `json:"name"`
	URL          string           `json:"url,omitempty"`
	DatabaseID   string           `json:"databaseId"`
	DataSourceID string           `json:"dataSourceId"`
	Properties   []schemaProperty `json:"properties"`
}

var settableTypes = map[string]bool{
	"rich_text": true, "number": true, "checkbox": true, "url": true, "email": true,
	"phone_number": true, "date": true, "select": true, "status": true, "multi_select": true,
}

func describeSchema(d dataSource) schemaInfo {
	info := schemaInfo{Name: d.name(), URL: d.DatabaseURL, DatabaseID: d.Parent.DatabaseID, DataSourceID: d.ID}
	for name, p := range d.Properties {
		sp := schemaProperty{Name: name, Type: p.Type, Settable: "no"}
		var opts *dsOptions
		switch p.Type {
		case "select":
			opts = p.Select
		case "multi_select":
			opts = p.MultiSelect
		case "status":
			opts = p.Status
		}
		if opts != nil {
			for _, o := range opts.Options {
				sp.Options = append(sp.Options, o.Name)
			}
		}
		switch {
		case p.Type == "title":
			sp.Settable = "title"
			sp.Note = "set with --title or the doc's H1"
		case settableTypes[p.Type]:
			sp.Settable = "yes"
			if opts != nil {
				sp.Note = "existing options only"
			}
		case p.Type == "relation" && p.Relation != nil && p.Relation.Type == "single_property":
			sp.Settable = "yes"
			sp.Note = "one-way relation: page URLs or IDs from data source " + p.Relation.DataSourceID
		case p.Type == "relation":
			sp.Note = "two-way relation: set by hand in Notion"
		default:
			sp.Note = "read-only in Notion"
		}
		info.Properties = append(info.Properties, sp)
	}
	sort.Slice(info.Properties, func(i, j int) bool {
		a, b := info.Properties[i], info.Properties[j]
		if (a.Type == "title") != (b.Type == "title") {
			return a.Type == "title"
		}
		return a.Name < b.Name
	})
	return info
}

func (s schemaInfo) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n", s.Name)
	if s.URL != "" {
		fmt.Fprintf(&b, "  url          %s\n", s.URL)
	}
	fmt.Fprintf(&b, "  database     %s\n  data source  %s\n\n", s.DatabaseID, s.DataSourceID)
	for _, p := range s.Properties {
		mark := map[string]string{"yes": "✓", "title": "✓", "no": "✗"}[p.Settable]
		fmt.Fprintf(&b, "%s %s (%s)", mark, p.Name, p.Type)
		if p.Note != "" {
			fmt.Fprintf(&b, ": %s", p.Note)
		}
		b.WriteString("\n")
		if len(p.Options) > 0 {
			fmt.Fprintf(&b, "    options: %s\n", strings.Join(p.Options, ", "))
		}
	}
	b.WriteString("\n✓ can be set with mdv notion push --prop \"Name=value\" (title: --title)\n")
	return b.String()
}

func notionSchemaCmd(args []string) error {
	fs := flag.NewFlagSet("schema", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print JSON")
	arg, err := parseFileArgs(fs, args)
	if err != nil {
		return errors.New("schema takes one database URL or ID (see mdv notion --help)")
	}
	d, err := resolveDataSource(arg)
	if err != nil {
		return err
	}
	return printReadResult(describeSchema(d), *asJSON)
}

type pageInfo struct {
	Title          string `json:"title"`
	URL            string `json:"url"`
	ID             string `json:"id"`
	ParentType     string `json:"parentType"`
	ParentID       string `json:"parentId,omitempty"`
	LastEditedTime string `json:"lastEditedTime"`
	InTrash        bool   `json:"inTrash"`
	CreatedByMdv   bool   `json:"createdByMdv"`
	Doc            string `json:"doc,omitempty"`
	SyncState      string `json:"syncState,omitempty"`
}

func describePage(dir string, p notionPage, state notionState) pageInfo {
	info := pageInfo{
		Title: p.title(), URL: p.URL, ID: p.ID, ParentType: p.Parent.Type,
		LastEditedTime: p.LastEditedTime, InTrash: p.InTrash,
	}
	switch p.Parent.Type {
	case "page_id":
		info.ParentID = p.Parent.PageID
	case "database_id":
		info.ParentID = p.Parent.DatabaseID
	case "data_source_id":
		info.ParentID = p.Parent.DataSourceID
	}
	for rel, e := range state {
		if e.PageID == p.ID {
			info.CreatedByMdv, info.Doc = true, rel
			info.SyncState = state.status(dir, rel, filepath.Join(dir, rel)).String()
			break
		}
	}
	return info
}

func (p pageInfo) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n  url          %s\n  id           %s\n", p.Title, p.URL, p.ID)
	parent := p.ParentType
	if p.ParentID != "" {
		parent += " " + p.ParentID
	}
	fmt.Fprintf(&b, "  parent       %s\n  last edited  %s\n", parent, p.LastEditedTime)
	if p.InTrash {
		b.WriteString("  in trash     yes\n")
	}
	if p.CreatedByMdv {
		fmt.Fprintf(&b, "  created by   mdv, from %s (%s)\n", p.Doc, p.SyncState)
	} else {
		b.WriteString("  created by   not mdv: mdv won't write to this page\n")
	}
	return b.String()
}

func notionInfoCmd(dir string, args []string) error {
	fs := flag.NewFlagSet("info", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print JSON")
	arg, err := parseFileArgs(fs, args)
	if err != nil {
		return errors.New("info takes one page URL or ID (see mdv notion --help)")
	}
	id, err := parsePageID(arg)
	if err != nil {
		return err
	}
	page, err := getPage(id)
	if err != nil {
		return err
	}
	state, err := loadNotionState(dir)
	if err != nil {
		return err
	}
	return printReadResult(describePage(dir, page, state), *asJSON)
}

func printReadResult(v fmt.Stringer, asJSON bool) error {
	if asJSON {
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}
	fmt.Print(v.String())
	return nil
}
