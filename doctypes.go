package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

type docType struct {
	name     string
	emoji    string
	sections []string
}

var docTypes = []docType{
	{"investigation", "🔍", []string{"Question", "Findings", "Options considered", "Recommendation", "Open questions", "For future work"}},
	{"plan", "📋", []string{"Goal", "Approach", "Steps", "Risks", "Out of scope", "For future work"}},
	{"summary", "✅", []string{"What changed", "Why", "How to verify", "Caveats", "Follow-ups", "For future work"}},
	{"bug", "🐛", []string{"Symptom", "Root cause", "Fix", "How to verify", "Related areas at risk", "For future work"}},
	{"how-to", "🛠️", []string{"When to use", "Steps", "Troubleshooting"}},
	{"reference", "📖", []string{"Overview", "Details", "Gotchas"}},
}

var docStatuses = []string{"in-progress", "done", "abandoned", "superseded"}

func lookupDocType(name string) (docType, bool) {
	for _, t := range docTypes {
		if t.name == name {
			return t, true
		}
	}
	return docType{}, false
}

func docTypeNames() []string {
	var names []string
	for _, t := range docTypes {
		names = append(names, t.name)
	}
	return names
}

const newUsage = `Usage:
  mdv new <type> [TICKET] <slug> [flags]   start a doc from the template for <type>

  types: investigation, plan, summary, bug, how-to, reference

  --repo <R>    repo folder name (default: the git repo you're in)
  --app <A>     app or package (default: from apps/<A> or packages/<A> if you're in one)
  --title <T>   H1 title (default: from the slug)
  --print       print the doc instead of writing it

Writes {docs}/{repo}/[{app}/]TICKET-slug.md with an agent index (ticket, repo,
app, branch and dates filled in) and the sections for <type>. It never
overwrites an existing file. Fill it in, then run mdv check on it.`

func newCmd(dir string, args []string) error {
	fs := flag.NewFlagSet("new", flag.ContinueOnError)
	repo := fs.String("repo", "", "repo")
	app := fs.String("app", "", "app or package")
	title := fs.String("title", "", "title")
	printOnly := fs.Bool("print", false, "print instead of writing")
	fs.Usage = func() { fmt.Println(newUsage) }
	var pos []string
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
		pos = append(pos, fs.Arg(0))
		args = fs.Args()[1:]
	}
	if len(pos) < 2 || len(pos) > 3 {
		return errors.New("new needs <type> [TICKET] <slug> (see mdv new --help)")
	}
	t, ok := lookupDocType(pos[0])
	if !ok {
		return fmt.Errorf("unknown type %q; use one of: %s", pos[0], strings.Join(docTypeNames(), ", "))
	}
	ticket, slug := "", pos[len(pos)-1]
	if len(pos) == 3 {
		ticket = strings.ToUpper(pos[1])
	} else if m := ticketInName.FindStringSubmatch(slug); m != nil && m[0] == slug {
		return errors.New("new needs a slug after the ticket, e.g. mdv new summary TAP-123 fetch-header-bug")
	}
	slug = kebab(slug)
	if slug == "" {
		return errors.New("the slug is empty")
	}

	gitRepo, gitApp, branch := gitContext()
	if *repo == "" {
		*repo = gitRepo
		if *app == "" {
			*app = gitApp
		}
	}
	if *repo == "" {
		return errors.New("not in a git repo; pass --repo (and --app for a monorepo app)")
	}
	if *title == "" {
		*title = humanize(slug)
	}
	name := slug + ".md"
	if ticket != "" {
		name = ticket + "-" + name
	}
	rel := filepath.Join(*repo, *app, name)
	content := newDocContent(t, ticket, *repo, *app, branch, *title, time.Now().Format("2006-01-02"))
	if *printOnly {
		fmt.Print(content)
		return nil
	}
	path := filepath.Join(dir, rel)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s already exists; update it instead of starting a new doc", rel)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return err
	}
	fmt.Println("Created", path)
	return nil
}

func newDocContent(t docType, ticket, repo, app, branch, title, today string) string {
	var b strings.Builder
	b.WriteString("---\n")
	if ticket != "" {
		fmt.Fprintf(&b, "ticket: %s\n", ticket)
	}
	fmt.Fprintf(&b, "repo: %s\n", repo)
	if app != "" {
		fmt.Fprintf(&b, "apps: [%s]\n", app)
	}
	fmt.Fprintf(&b, "type: %s\nstatus: in-progress\nsummary: \"\"\nkeywords: []\npaths: []\n", t.name)
	if branch != "" {
		fmt.Fprintf(&b, "branch: %s\n", branch)
	}
	fmt.Fprintf(&b, "pr:\ncaveats: []\nrelated: []\ncreated: %s\nupdated: %s\n---\n", today, today)
	h1 := title
	if ticket != "" {
		h1 = ticket + ": " + title
	}
	fmt.Fprintf(&b, "# %s %s\n", t.emoji, h1)
	for _, s := range t.sections {
		fmt.Fprintf(&b, "\n## %s\n", s)
	}
	return b.String()
}

func gitContext() (repo, app, branch string) {
	top, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", "", ""
	}
	root := strings.TrimSpace(string(top))
	repo = filepath.Base(root)
	if cwd, err := os.Getwd(); err == nil {
		if real, err := filepath.EvalSymlinks(cwd); err == nil {
			cwd = real
		}
		if realRoot, err := filepath.EvalSymlinks(root); err == nil {
			root = realRoot
		}
		if rel, err := filepath.Rel(root, cwd); err == nil {
			parts := strings.Split(filepath.ToSlash(rel), "/")
			if len(parts) >= 2 && (parts[0] == "apps" || parts[0] == "packages") {
				app = parts[1]
			}
		}
	}
	if out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
		if b := strings.TrimSpace(string(out)); b != "HEAD" && b != "main" && b != "master" {
			branch = b
		}
	}
	return repo, app, branch
}

func kebab(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(strings.TrimSuffix(s, ".md")) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func humanize(slug string) string {
	s := strings.ReplaceAll(slug, "-", " ")
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
