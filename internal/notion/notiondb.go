package notion

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jakobluthman95/mdv/internal/docs"
)

// Database support: mdv can add a doc as a new entry (page) in an existing
// Notion database. It only ever creates entries and edits the entries it
// created; it never creates, renames or changes a database or its properties.
//
// Notion databases hold one or more data sources, each with its own property
// schema. New entries are created under a data source.

type dataSource struct {
	ID    string `json:"id"`
	Title []struct {
		PlainText string `json:"plain_text"`
	} `json:"title"`
	Parent struct {
		DatabaseID string `json:"database_id"`
	} `json:"parent"`
	Properties map[string]dsProperty `json:"properties"`
	// DatabaseURL is filled in when the database itself was read.
	DatabaseURL string `json:"-"`
}

type dsOption struct {
	Name string `json:"name"`
}

type dsOptions struct {
	Options []dsOption `json:"options"`
}

type dsProperty struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Type        string     `json:"type"`
	Select      *dsOptions `json:"select"`
	MultiSelect *dsOptions `json:"multi_select"`
	Status      *dsOptions `json:"status"`
	Relation    *struct {
		DataSourceID string `json:"data_source_id"`
		Type         string `json:"type"`
	} `json:"relation"`
}

func (d dataSource) name() string {
	var b strings.Builder
	for _, t := range d.Title {
		b.WriteString(t.PlainText)
	}
	if b.Len() == 0 {
		return "Untitled database"
	}
	return b.String()
}

func getDataSource(id string) (dataSource, error) {
	var d dataSource
	out, err := notionRequest("GET", "v1/data_sources/"+id, nil, nil)
	if err != nil {
		return d, err
	}
	if err := json.Unmarshal(out, &d); err != nil {
		return d, err
	}
	if d.ID == "" {
		return d, fmt.Errorf("reading data source %s returned no ID: %.200s", id, out)
	}
	return d, nil
}

// resolveDataSource accepts a database URL or ID (also a database view URL) or
// a data source ID, and returns the data source new entries are created under.
// It only reads.
func resolveDataSource(arg string) (dataSource, error) {
	id, err := parsePageID(arg)
	if err != nil {
		return dataSource{}, fmt.Errorf("cannot find a Notion database ID in %q", arg)
	}
	out, dbErr := notionRequest("GET", "v1/databases/"+id, nil, nil)
	if dbErr != nil {
		d, err := getDataSource(id)
		if err != nil {
			return dataSource{}, fmt.Errorf("cannot read database %s: %w", id, dbErr)
		}
		return d, nil
	}
	var db struct {
		URL         string `json:"url"`
		InTrash     bool   `json:"in_trash"`
		DataSources []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"data_sources"`
	}
	if err := json.Unmarshal(out, &db); err != nil {
		return dataSource{}, err
	}
	if db.InTrash {
		return dataSource{}, fmt.Errorf("database %s is in the trash", id)
	}
	switch len(db.DataSources) {
	case 0:
		return dataSource{}, fmt.Errorf("database %s has no data sources to add entries to", id)
	case 1:
		d, err := getDataSource(db.DataSources[0].ID)
		d.DatabaseURL = db.URL
		return d, err
	}
	var names []string
	for _, s := range db.DataSources {
		names = append(names, fmt.Sprintf("%q (%s)", s.Name, s.ID))
	}
	return dataSource{}, fmt.Errorf("database %s has %d data sources: %s; pass the ID of the one to use with --database", id, len(db.DataSources), strings.Join(names, ", "))
}

// propFlags collects repeated --prop "Name=value" flags.
type propFlags []string

func (p *propFlags) String() string { return strings.Join(*p, ", ") }

func (p *propFlags) Set(v string) error {
	if !strings.Contains(v, "=") {
		return fmt.Errorf("--prop needs Name=value, got %q", v)
	}
	*p = append(*p, v)
	return nil
}

// buildProps turns --prop flags into a Notion properties object for a page in
// d. Values are checked against the schema: option values must already exist,
// because Notion would otherwise add them to the database. Relations are only
// allowed when they are one-way, so linking never changes another page.
// getPageFn reads a relation target (only reads).
func buildProps(d dataSource, raw []string, getPageFn func(string) (notionPage, error)) (map[string]any, []string, error) {
	props := map[string]any{}
	var summary []string
	for _, r := range raw {
		name, value, _ := strings.Cut(r, "=")
		name, value = strings.TrimSpace(name), strings.TrimSpace(value)
		prop, err := findProperty(d, name)
		if err != nil {
			return nil, nil, err
		}
		if _, dup := props[prop.Name]; dup {
			return nil, nil, fmt.Errorf("--prop %q is given more than once", prop.Name)
		}
		v, shown, err := propValue(prop, value, getPageFn)
		if err != nil {
			return nil, nil, fmt.Errorf("--prop %q: %w", prop.Name, err)
		}
		props[prop.Name] = v
		summary = append(summary, prop.Name+"="+shown)
	}
	return props, summary, nil
}

func findProperty(d dataSource, name string) (dsProperty, error) {
	if p, ok := d.Properties[name]; ok {
		p.Name = name
		return p, nil
	}
	var match []string
	for n := range d.Properties {
		if strings.EqualFold(n, name) || strings.EqualFold(docs.StripEmoji(n), name) {
			match = append(match, n)
		}
	}
	if len(match) == 1 {
		p := d.Properties[match[0]]
		p.Name = match[0]
		return p, nil
	}
	var names []string
	for n, p := range d.Properties {
		names = append(names, fmt.Sprintf("%s (%s)", n, p.Type))
	}
	sort.Strings(names)
	return dsProperty{}, fmt.Errorf("%q has no property %q; it has: %s", d.name(), name, strings.Join(names, ", "))
}

func propValue(p dsProperty, value string, getPageFn func(string) (notionPage, error)) (any, string, error) {
	switch p.Type {
	case "title":
		return nil, "", fmt.Errorf("this is the title; use --title instead")
	case "rich_text":
		return map[string]any{"rich_text": []any{map[string]any{"type": "text", "text": map[string]string{"content": value}}}}, value, nil
	case "number":
		n, err := strconv.ParseFloat(value, 64)
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
			return nil, "", fmt.Errorf("%q is not a number", value)
		}
		return map[string]any{"number": n}, value, nil
	case "checkbox":
		switch strings.ToLower(value) {
		case "true", "yes", "1":
			return map[string]any{"checkbox": true}, "true", nil
		case "false", "no", "0":
			return map[string]any{"checkbox": false}, "false", nil
		}
		return nil, "", fmt.Errorf("%q is not true or false", value)
	case "url", "email", "phone_number":
		if value == "" {
			return map[string]any{p.Type: nil}, "", nil
		}
		return map[string]any{p.Type: value}, value, nil
	case "date":
		if _, err := time.Parse("2006-01-02", value); err != nil {
			if _, err := time.Parse(time.RFC3339, value); err != nil {
				return nil, "", fmt.Errorf("%q is not a date (YYYY-MM-DD or RFC 3339)", value)
			}
		}
		return map[string]any{"date": map[string]string{"start": value}}, value, nil
	case "select", "status":
		opts := p.Select
		if p.Type == "status" {
			opts = p.Status
		}
		o, err := matchOption(opts, value)
		if err != nil {
			return nil, "", err
		}
		return map[string]any{p.Type: map[string]string{"name": o}}, o, nil
	case "multi_select":
		var picked []any
		var names []string
		for _, v := range splitList(value) {
			o, err := matchOption(p.MultiSelect, v)
			if err != nil {
				return nil, "", err
			}
			picked = append(picked, map[string]string{"name": o})
			names = append(names, o)
		}
		return map[string]any{"multi_select": picked}, strings.Join(names, ", "), nil
	case "relation":
		if p.Relation == nil || p.Relation.Type != "single_property" {
			return nil, "", fmt.Errorf("this is a two-way relation: linking would also change the linked pages, which mdv doesn't do; set it by hand in Notion")
		}
		var ids []any
		var titles []string
		for _, v := range splitList(value) {
			id, err := parsePageID(v)
			if err != nil {
				return nil, "", err
			}
			page, err := getPageFn(id)
			if err != nil {
				return nil, "", fmt.Errorf("cannot read linked page %s: %w", id, err)
			}
			if page.Parent.DataSourceID != p.Relation.DataSourceID {
				return nil, "", fmt.Errorf("page %s is not in the database this relation links to", id)
			}
			ids = append(ids, map[string]string{"id": page.ID})
			titles = append(titles, page.title())
		}
		return map[string]any{"relation": ids}, strings.Join(titles, ", "), nil
	}
	return nil, "", fmt.Errorf("properties of type %s can't be set with --prop", p.Type)
}

// matchOption finds an existing option: exact, then ignoring case, then also
// ignoring a leading emoji ("Technical Documentation" matches
// "⚙️ Technical Documentation").
func matchOption(opts *dsOptions, value string) (string, error) {
	var names []string
	if opts != nil {
		for _, o := range opts.Options {
			names = append(names, o.Name)
		}
	}
	for _, same := range []func(string) bool{
		func(n string) bool { return n == value },
		func(n string) bool { return strings.EqualFold(n, value) },
		func(n string) bool { return strings.EqualFold(docs.StripEmoji(n), docs.StripEmoji(value)) },
	} {
		var hits []string
		for _, n := range names {
			if same(n) {
				hits = append(hits, n)
			}
		}
		if len(hits) == 1 {
			return hits[0], nil
		}
		if len(hits) > 1 {
			return "", fmt.Errorf("%q matches several options: %s", value, strings.Join(hits, ", "))
		}
	}
	return "", fmt.Errorf("%q is not an existing option (mdv doesn't add options to databases); options: %s", value, strings.Join(names, ", "))
}

func splitList(value string) []string {
	var out []string
	for _, v := range strings.Split(value, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}
