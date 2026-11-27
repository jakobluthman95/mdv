package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"
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
Exits with status 1 when any doc has errors.`

type checkIssue struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

type checkResult struct {
	Path   string       `json:"path"`
	Issues []checkIssue `json:"issues"`
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
	return strings.ToLower(strings.TrimSpace(stripEmoji(strings.TrimSpace(h))))
}

func checkDoc(content []byte, now time.Time) []checkIssue {
	var issues []checkIssue
	add := func(level, format string, a ...any) {
		issues = append(issues, checkIssue{level, fmt.Sprintf(format, a...)})
	}
	meta, body, ok, err := splitFrontmatter(content)
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
	t, known := lookupDocType(meta.Type)
	if meta.Type != "" && !known {
		add("error", "agent index: unknown type %q; use one of: %s", meta.Type, strings.Join(docTypeNames(), ", "))
	}
	if meta.Status != "" && !slices.Contains(docStatuses, meta.Status) {
		add("error", "agent index: unknown status %q; use one of: %s", meta.Status, strings.Join(docStatuses, ", "))
	}
	if meta.Status == "superseded" && meta.SupersededBy == "" {
		add("warning", "agent index: status is superseded but superseded_by is empty")
	}
	if meta.Status == "in-progress" {
		if d, err := time.Parse("2006-01-02", meta.Updated); err == nil && now.Sub(d) > 30*24*time.Hour {
			add("warning", "in-progress but not updated since %s; update status, pr and updated", meta.Updated)
		}
	}

	_, title, _ := splitTitle(body)
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
		if nfmFence.MatchString(line) {
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
		for _, s := range t.sections {
			key := normalizeHeading(s)
			content, present := sections[key]
			switch {
			case !present:
				add("error", "missing section \"## %s\" (type %s)", s, t.name)
			case meta.Status == "done" && strings.TrimSpace(content) == "":
				add("warning", "section \"## %s\" is empty in a done doc; fill it in or write \"None.\"", s)
			}
		}
	}
	return issues
}

func checkCmd(dir string, args []string) error {
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
		for _, f := range markdownFiles(dir) {
			b, err := os.ReadFile(f.path)
			if err != nil {
				continue
			}
			if _, _, ok, _ := splitFrontmatter(b); !ok {
				skipped++
				continue
			}
			targets = append(targets, target{f.path, f.rel})
		}
	}
	for _, f := range files {
		path, rel, err := resolveDoc(dir, f)
		if err != nil {
			return err
		}
		targets = append(targets, target{path, rel})
	}

	var results []checkResult
	failed := false
	for _, tg := range targets {
		b, err := os.ReadFile(tg.path)
		if err != nil {
			return err
		}
		r := checkResult{Path: tg.rel, Issues: checkDoc(b, time.Now())}
		if r.Issues == nil {
			r.Issues = []checkIssue{}
		}
		failed = failed || r.errors() > 0
		results = append(results, r)
	}

	if *asJSON {
		if results == nil {
			results = []checkResult{}
		}
		if err := printJSON(results); err != nil {
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
			fmt.Println(dimStyle.Render(fmt.Sprintf("%d docs without an agent index were skipped (mdv find --unindexed)", skipped)))
		}
	}
	if failed {
		return errCheckFailed
	}
	return nil
}

var errCheckFailed = errors.New("some docs have errors")

func checkWarnings(path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	if _, _, ok, _ := splitFrontmatter(b); !ok {
		return
	}
	if n := len(checkDoc(b, time.Now())); n > 0 {
		fmt.Fprintf(os.Stderr, "⚠ mdv check found %d issue(s) in %s; run mdv check on it\n", n, tildify(path))
	}
}
