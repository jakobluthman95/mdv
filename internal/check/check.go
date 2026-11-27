package check

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/jakobluthman95/mdv/internal/docs"
	"github.com/jakobluthman95/mdv/internal/render"
	"github.com/jakobluthman95/mdv/internal/search"
	"github.com/jakobluthman95/mdv/internal/term"
)

const checkUsage = `Usage:
  mdv check <file.md>…        check docs against the agent-index and template rules (read-only)
  mdv check --all             check every doc that has an agent index

  --json   print JSON

Errors: missing or invalid agent index, missing required fields, unknown type
or status, missing sections for the doc's type. Warnings: empty sections in a
done doc, relative links, wording that only makes sense in the moment
("yesterday", "as discussed"), a summary that just repeats the title, and an
in-progress doc that hasn't been updated for 30 days.
Regressions: a regressed status needs issues, and each issue a ticket and a
found date (errors); builds_on entries that match no doc, or that point at
work that regressed or has open issues, and a done summary or bug without
verification are warnings.
Exits with status 1 when any doc has errors.`

type Issue struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

type checkResult struct {
	Path   string  `json:"path"`
	Issues []Issue `json:"issues"`
}

func (r checkResult) errors() int {
	n := 0
	for _, i := range r.Issues {
		if i.Level == "error" {
			n++
		}
	}
	return n
}

var (
	h2Line         = regexp.MustCompile(`^##\s+(.+?)\s*$`)
	relativeLink   = regexp.MustCompile(`(^|[^!])\[[^\]]*\]\(([^)\s]+)\)`)
	momentaryWords = regexp.MustCompile(`(?i)\b(yesterday|tomorrow|last week|next week|this morning|as discussed|as mentioned|see above|see below|the above|mentioned above|discussed earlier)\b`)
	codeSpan       = regexp.MustCompile("`[^`]*`")
)

func normalizeHeading(h string) string {
	return strings.ToLower(strings.TrimSpace(docs.StripEmoji(strings.TrimSpace(h))))
}

// Doc checks one doc on its own. DocIn also checks what it builds on.
func Doc(content []byte, now time.Time) []Issue {
	return DocIn(nil, "", content, now)
}

// DocIn checks one doc against the other docs in the folder: all is the
// folder's docs (search.LoadDocs) and path the doc's absolute path.
func DocIn(all []search.Doc, path string, content []byte, now time.Time) []Issue {
	var issues []Issue
	add := func(level, format string, a ...any) {
		issues = append(issues, Issue{level, fmt.Sprintf(format, a...)})
	}
	meta, body, ok, err := docs.SplitFrontmatter(content)
	switch {
	case err != nil:
		add("error", "%v", err)
		return issues
	case !ok:
		add("error", "no agent index (YAML frontmatter); mdv new <type> … starts a doc with one")
		return issues
	}

	for name, v := range map[string]string{"repo": meta.Repo, "type": meta.Type, "status": meta.Status, "summary": strings.TrimSpace(meta.Summary), "updated": meta.Updated} {
		if v == "" {
			add("error", "agent index: %s is required", name)
		}
	}
	if len(meta.Keywords) == 0 {
		add("error", "agent index: keywords is required (terms someone would search for)")
	}
	t, known := docs.LookupType(meta.Type)
	if meta.Type != "" && !known {
		add("error", "agent index: unknown type %q; use one of: %s", meta.Type, strings.Join(docs.TypeNames(), ", "))
	}
	if meta.Status != "" && !slices.Contains(docs.Statuses, meta.Status) {
		add("error", "agent index: unknown status %q; use one of: %s", meta.Status, strings.Join(docs.Statuses, ", "))
	}
	if meta.Status == "superseded" && meta.SupersededBy == "" {
		add("warning", "agent index: status is superseded but superseded_by is empty")
	}
	if meta.Status == "regressed" && len(meta.Issues) == 0 {
		add("error", "agent index: status is regressed but issues is empty; record the ticket and the date it was found")
	}
	for n, i := range meta.Issues {
		if strings.TrimSpace(i.Ticket) == "" {
			add("error", "agent index: issues entry %d has no ticket", n+1)
		}
		if strings.TrimSpace(i.Found) == "" {
			add("error", "agent index: issues entry %d has no found date", n+1)
		}
	}
	if meta.Status == "done" && (meta.Type == "summary" || meta.Type == "bug") && len(meta.Verification) == 0 {
		add("warning", "agent index: verification is empty in a done %s; list how the work was verified", meta.Type)
	}
	if all != nil {
		for _, ref := range search.Unresolved(all, meta) {
			add("warning", "agent index: builds_on %s matches no doc in the docs folder", ref)
		}
		for _, u := range search.RegressedUpstream(all, search.Doc{AbsPath: path, Meta: meta}) {
			add("warning", "%s; re-check this work", u.Message)
		}
	}
	if meta.Status == "in-progress" {
		if d, err := time.Parse("2006-01-02", meta.Updated); err == nil && now.Sub(d) > 30*24*time.Hour {
			add("warning", "in-progress but not updated since %s; update status, pr and updated", meta.Updated)
		}
	}

	_, title, _ := docs.SplitTitle(body)
	if title == "" {
		add("warning", "no # H1 title after the agent index")
	}
	summary := strings.Join(strings.Fields(meta.Summary), " ")
	if summary != "" && (len(summary) < 25 || strings.EqualFold(strings.Trim(summary, ". "), strings.TrimSpace(title))) {
		add("warning", "summary is too thin; say what was done or found, and how")
	}

	sections := map[string]string{}
	var order []string
	current := ""
	inFence := false
	for _, line := range strings.Split(string(body), "\n") {
		if render.Fence.MatchString(line) {
			inFence = !inFence
		}
		if !inFence {
			if m := h2Line.FindStringSubmatch(line); m != nil {
				current = normalizeHeading(m[1])
				order = append(order, current)
				sections[current] = ""
				continue
			}
		}
		if current != "" {
			sections[current] += line + "\n"
		}
		if inFence {
			continue
		}
		text := codeSpan.ReplaceAllString(line, "")
		for _, m := range relativeLink.FindAllStringSubmatch(text, -1) {
			if target := m[2]; !strings.Contains(target, "://") && !strings.HasPrefix(target, "#") && !strings.HasPrefix(target, "mailto:") {
				add("warning", "relative link %q breaks in Notion and gists; use an absolute URL, or write repo paths as `code`", target)
			}
		}
		if m := momentaryWords.FindString(text); m != "" {
			add("warning", "%q only makes sense in the moment; use a date, link or name", m)
		}
	}
	if known {
		for _, s := range t.Sections {
			key := normalizeHeading(s)
			content, present := sections[key]
			switch {
			case !present:
				add("error", "missing section \"## %s\" (type %s)", s, t.Name)
			case meta.Status == "done" && strings.TrimSpace(content) == "":
				add("warning", "section \"## %s\" is empty in a done doc; fill it in or write \"None.\"", s)
			}
		}
	}
	return issues
}

func Run(dir string, args []string) error {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	all := fs.Bool("all", false, "check every doc with an agent index")
	asJSON := fs.Bool("json", false, "print JSON")
	fs.Usage = func() { fmt.Println(checkUsage) }
	var files []string
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
		files = append(files, fs.Arg(0))
		args = fs.Args()[1:]
	}
	if !*all && len(files) == 0 {
		return errors.New("check needs a file or --all (see mdv check --help)")
	}

	type target struct{ path, rel string }
	var targets []target
	skipped := 0
	if *all {
		for _, f := range docs.MarkdownFiles(dir) {
			b, err := os.ReadFile(f.Path)
			if err != nil {
				continue
			}
			if _, _, ok, _ := docs.SplitFrontmatter(b); !ok {
				skipped++
				continue
			}
			targets = append(targets, target{f.Path, f.Rel})
		}
	}
	for _, f := range files {
		path, rel, err := docs.ResolveDoc(dir, f)
		if err != nil {
			return err
		}
		targets = append(targets, target{path, rel})
	}

	folder := search.LoadDocs(dir)
	var results []checkResult
	failed := false
	for _, tg := range targets {
		b, err := os.ReadFile(tg.path)
		if err != nil {
			return err
		}
		r := checkResult{Path: tg.rel, Issues: DocIn(folder, tg.path, b, time.Now())}
		if r.Issues == nil {
			r.Issues = []Issue{}
		}
		failed = failed || r.errors() > 0
		results = append(results, r)
	}

	if *asJSON {
		if results == nil {
			results = []checkResult{}
		}
		if err := term.PrintJSON(results); err != nil {
			return err
		}
	} else {
		for _, r := range results {
			if len(r.Issues) == 0 {
				fmt.Println("✓", r.Path)
				continue
			}
			fmt.Println("✗", r.Path)
			for _, i := range r.Issues {
				mark := "  error  "
				if i.Level == "warning" {
					mark = "  warning"
				}
				fmt.Println(mark, i.Message)
			}
		}
		if *all && skipped > 0 {
			fmt.Println(term.DimStyle.Render(fmt.Sprintf("%d docs without an agent index were skipped (mdv find --unindexed)", skipped)))
		}
	}
	if failed {
		return ErrFailed
	}
	return nil
}

var ErrFailed = errors.New("some docs have errors")

func Warnings(path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	if _, _, ok, _ := docs.SplitFrontmatter(b); !ok {
		return
	}
	var all []search.Doc
	if dir, err := docs.Dir(); err == nil {
		all = search.LoadDocs(dir)
	}
	if n := len(DocIn(all, path, b, time.Now())); n > 0 {
		fmt.Fprintf(os.Stderr, "⚠ mdv check found %d issue(s) in %s; run mdv check on it\n", n, term.Tildify(path))
	}
}
