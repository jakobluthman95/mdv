package search

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jakobluthman95/mdv/internal/docs"
	"github.com/jakobluthman95/mdv/internal/term"
)

const findUsage = `Usage:
  mdv find [terms…] [flags]   find docs by agent index and text, best match first (read-only)
  mdv find --unindexed        list docs without an agent index

  --ticket <T>   only docs for ticket T (or that list it under related)
  --repo <R>     only docs for repo R
  --app <A>      only docs for app or package A
  --status <S>   only docs with status S (in-progress, done, abandoned, superseded)
  --path <P>     only docs whose paths overlap P (or that mention it)
  --limit <N>    at most N results (default 10)
  --json         print JSON, for agents

Docs without an agent index still match: repo and app come from their folder
({repo}/{app}/…) and the ticket from the file name (TAP-1234-…).`

type findResult struct {
	Path     string    `json:"path"`
	AbsPath  string    `json:"absPath"`
	Title    string    `json:"title"`
	Indexed  bool      `json:"indexed"`
	Score    int       `json:"score"`
	Matched  []string  `json:"matched,omitempty"`
	Meta     docs.Meta `json:"meta"`
	IndexErr string    `json:"indexError,omitempty"`
	modTime  int64
	body     string
}

type findQuery struct {
	terms                         []string
	ticket, repo, app, status, pp string
}

func loadFindDocs(dir string) []findResult {
	var found []findResult
	for _, f := range docs.MarkdownFiles(dir) {
		b, err := os.ReadFile(f.Path)
		if err != nil {
			continue
		}
		meta, body, ok, ferr := docs.SplitFrontmatter(b)
		r := findResult{Path: f.Rel, AbsPath: f.Path, Indexed: ok && ferr == nil, Meta: meta, modTime: f.ModTime.Unix(), body: string(body)}
		if ferr != nil {
			r.IndexErr = ferr.Error()
		}
		parts := strings.Split(filepath.ToSlash(f.Rel), "/")
		if r.Meta.Repo == "" && len(parts) >= 2 {
			r.Meta.Repo = parts[0]
		}
		if len(r.Meta.Apps) == 0 && len(parts) >= 3 {
			r.Meta.Apps = []string{parts[1]}
		}
		if r.Meta.Ticket == "" {
			if m := docs.TicketInName.FindStringSubmatch(filepath.Base(f.Rel)); m != nil {
				r.Meta.Ticket = strings.ToUpper(m[1])
			}
		}
		r.Title = docs.Title(f.Rel, body)
		found = append(found, r)
	}
	return found
}

func containsFold(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}

func pathsOverlap(a, b string) bool {
	a, b = strings.Trim(strings.ToLower(a), "/"), strings.Trim(strings.ToLower(b), "/")
	return a != "" && b != "" && (strings.HasPrefix(a, b) || strings.HasPrefix(b, a))
}

func (q findQuery) keep(r *findResult) bool {
	m := r.Meta
	if q.ticket != "" && !strings.EqualFold(m.Ticket, q.ticket) && !containsFold(m.Related, q.ticket) {
		return false
	}
	if q.repo != "" && !strings.EqualFold(m.Repo, q.repo) {
		return false
	}
	if q.app != "" && !containsFold(m.Apps, q.app) {
		return false
	}
	if q.status != "" && !strings.EqualFold(m.Status, q.status) {
		return false
	}
	if q.pp != "" {
		hit := strings.Contains(strings.ToLower(r.body), strings.ToLower(strings.Trim(q.pp, "/")))
		for _, p := range m.Paths {
			hit = hit || pathsOverlap(p, q.pp)
		}
		if !hit {
			return false
		}
	}
	return true
}

func (q findQuery) score(r *findResult) {
	m := r.Meta
	add := func(n int, why string) {
		r.Score += n
		r.Matched = append(r.Matched, why)
	}
	if q.ticket != "" {
		if strings.EqualFold(m.Ticket, q.ticket) {
			add(100, "ticket")
		} else {
			add(60, "related:"+q.ticket)
		}
	}
	body := strings.ToLower(r.body)
	for _, t := range q.terms {
		lt := strings.ToLower(t)
		switch {
		case strings.EqualFold(m.Ticket, t):
			add(100, "ticket")
			continue
		case containsFold(m.Related, t):
			add(60, "related:"+t)
			continue
		}
		if containsFold(m.Keywords, t) {
			add(30, "keyword:"+t)
		} else {
			for _, k := range m.Keywords {
				if strings.Contains(strings.ToLower(k), lt) {
					add(15, "keyword~"+t)
					break
				}
			}
		}
		for _, p := range m.Paths {
			if strings.Contains(strings.ToLower(p), lt) {
				add(15, "path:"+t)
				break
			}
		}
		if strings.Contains(strings.ToLower(r.Title), lt) {
			add(20, "title:"+t)
		}
		if strings.Contains(strings.ToLower(m.Summary), lt) {
			add(15, "summary:"+t)
		}
		for _, c := range m.Caveats {
			if strings.Contains(strings.ToLower(c), lt) {
				add(10, "caveat:"+t)
				break
			}
		}
		if n := strings.Count(body, lt); n > 0 {
			add(min(n, 5)*2, fmt.Sprintf("text:%s×%d", t, n))
		}
	}
}

func findDocs(dir string, q findQuery, limit int) []findResult {
	var out []findResult
	for _, r := range loadFindDocs(dir) {
		if !q.keep(&r) {
			continue
		}
		q.score(&r)
		if len(q.terms) > 0 && r.Score == 0 {
			continue
		}
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].modTime > out[j].modTime
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func Run(dir string, args []string) error {
	fs := flag.NewFlagSet("find", flag.ContinueOnError)
	var q findQuery
	fs.StringVar(&q.ticket, "ticket", "", "ticket")
	fs.StringVar(&q.repo, "repo", "", "repo")
	fs.StringVar(&q.app, "app", "", "app or package")
	fs.StringVar(&q.status, "status", "", "status")
	fs.StringVar(&q.pp, "path", "", "path")
	limit := fs.Int("limit", 10, "max results")
	asJSON := fs.Bool("json", false, "print JSON")
	unindexed := fs.Bool("unindexed", false, "list docs without an agent index")
	fs.Usage = func() { fmt.Println(findUsage) }
	for len(args) > 0 {
		if err := fs.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return nil
			}
			return err
		}
		if fs.NArg() == 0 {
			break
		}
		q.terms = append(q.terms, fs.Arg(0))
		args = fs.Args()[1:]
	}

	if *unindexed {
		var paths []string
		for _, r := range loadFindDocs(dir) {
			if !r.Indexed {
				paths = append(paths, r.Path)
			}
		}
		sort.Strings(paths)
		if *asJSON {
			return term.PrintJSON(paths)
		}
		for _, p := range paths {
			fmt.Println(p)
		}
		return nil
	}

	results := findDocs(dir, q, *limit)
	if *asJSON {
		if results == nil {
			results = []findResult{}
		}
		return term.PrintJSON(results)
	}
	if len(results) == 0 {
		fmt.Println("No matching docs.")
		return nil
	}
	for i, r := range results {
		fmt.Printf("%2d. %s", i+1, r.Path)
		if r.Score > 0 {
			fmt.Print(term.DimStyle.Render(fmt.Sprintf("  (score %d)", r.Score)))
		}
		fmt.Println()
		fmt.Println("    " + r.Title)
		var facts []string
		for _, s := range []string{r.Meta.Ticket, r.Meta.Type, r.Meta.Status, r.Meta.PR.String()} {
			if s != "" {
				facts = append(facts, s)
			}
		}
		if !r.Indexed {
			facts = append(facts, "no agent index")
		}
		if len(facts) > 0 {
			fmt.Println("    " + term.DimStyle.Render(strings.Join(facts, " · ")))
		}
		if s := strings.Join(strings.Fields(r.Meta.Summary), " "); s != "" {
			fmt.Println("    " + s)
		}
		if r.Meta.SupersededBy != "" {
			fmt.Println("    → superseded by " + r.Meta.SupersededBy)
		}
		if len(r.Meta.Caveats) > 0 {
			fmt.Printf("    ⚠ %d caveat(s)\n", len(r.Meta.Caveats))
		}
		if len(r.Matched) > 0 {
			fmt.Println("    " + term.DimStyle.Render("matched: "+strings.Join(r.Matched, ", ")))
		}
	}
	return nil
}
