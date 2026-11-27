package setup

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/jakobluthman95/mdv/internal/docs"
	"github.com/jakobluthman95/mdv/internal/gitx"
)

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

func New(dir string, args []string) error {
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
	t, ok := docs.LookupType(pos[0])
	if !ok {
		return fmt.Errorf("unknown type %q; use one of: %s", pos[0], strings.Join(docs.TypeNames(), ", "))
	}
	ticket, slug := "", pos[len(pos)-1]
	if len(pos) == 3 {
		ticket = strings.ToUpper(pos[1])
	} else if m := docs.TicketInName.FindStringSubmatch(slug); m != nil && m[0] == slug {
		return errors.New("new needs a slug after the ticket, e.g. mdv new summary TAP-123 fetch-header-bug")
	}
	slug = kebab(slug)
	if slug == "" {
		return errors.New("the slug is empty")
	}

	gitRepo, gitApp, branch := gitx.Context()
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

func newDocContent(t docs.Type, ticket, repo, app, branch, title, today string) string {
	var b strings.Builder
	b.WriteString("---\n")
	if ticket != "" {
		fmt.Fprintf(&b, "ticket: %s\n", ticket)
	}
	fmt.Fprintf(&b, "repo: %s\n", repo)
	if app != "" {
		fmt.Fprintf(&b, "apps: [%s]\n", app)
	}
	fmt.Fprintf(&b, "type: %s\nstatus: in-progress\nsummary: \"\"\nkeywords: []\npaths: []\n", t.Name)
	if branch != "" {
		fmt.Fprintf(&b, "branch: %s\n", branch)
	}
	fmt.Fprintf(&b, "pr:\ncaveats: []\nrelated: []\ncreated: %s\nupdated: %s\n---\n", today, today)
	h1 := title
	if ticket != "" {
		h1 = ticket + ": " + title
	}
	fmt.Fprintf(&b, "# %s %s\n", t.Emoji, h1)
	for _, s := range t.Sections {
		fmt.Fprintf(&b, "\n## %s\n", s)
	}
	return b.String()
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
