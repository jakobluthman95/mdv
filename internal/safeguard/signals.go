package safeguard

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jakobluthman95/mdv/internal/docs"
	"github.com/jakobluthman95/mdv/internal/gitx"
	"github.com/jakobluthman95/mdv/internal/search"
	"github.com/jakobluthman95/mdv/internal/term"
)

const signalsUsage = `Usage:
  mdv signals <ticket|doc> [--json]   look for signs that earlier work regressed (read-only)

Checks, since the doc's merged date (or updated, or created): open issues and
a regressed status in its agent index; fix, revert and bug commits on its
paths and revert commits naming its ticket in the local repo
($MDV_REPOS_DIR/<repo>, default ~/dev/<repo>); revert PRs on GitHub (gh);
and docs it builds on (builds_on) that regressed. Jira and Sentry are not
checked. Exits 0 even when it finds warning signs.`

const signalsNote = "Jira and Sentry are not checked by mdv: check them with your connectors, or tell the user they couldn't be checked."

var fixSubject = regexp.MustCompile(`(?i)\b(fix|fixes|fixed|revert|hotfix|regression|bug)\b`)

var (
	lookPath = exec.LookPath
	runGH    = func(dir string, args ...string) ([]byte, error) { return gitx.Run(dir, nil, "gh", args...) }
)

type Signal struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
	Ref     string `json:"ref,omitempty"`
}

type NotChecked struct {
	What   string `json:"what"`
	Reason string `json:"reason"`
}

type SignalsReport struct {
	Doc        string       `json:"doc"`
	Ticket     string       `json:"ticket,omitempty"`
	Since      string       `json:"since"`
	SinceFrom  string       `json:"sinceFrom"`
	Warnings   []Signal     `json:"warnings"`
	Checked    []string     `json:"checked"`
	NotChecked []NotChecked `json:"notChecked"`
	Note       string       `json:"note"`
}

func ReposDir() (string, error) {
	if d := os.Getenv("MDV_REPOS_DIR"); d != "" {
		return docs.ExpandPath(d)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "dev"), nil
}

// resolveOne finds exactly one doc for ref, or explains why it can't.
func resolveOne(all []search.Doc, ref string) (search.Doc, error) {
	found := search.Resolve(all, ref)
	switch len(found) {
	case 0:
		return search.Doc{}, fmt.Errorf("no doc found for %s (pass a ticket or a path in the docs folder)", ref)
	case 1:
		return found[0], nil
	}
	lines := []string{fmt.Sprintf("%d docs match %s; pass a path instead:", len(found), ref)}
	for _, d := range found {
		lines = append(lines, "  "+d.Path)
	}
	return search.Doc{}, errors.New(strings.Join(lines, "\n"))
}

func parseRefArgs(name, usage string, args []string) (string, bool, error) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "print JSON")
	fs.Usage = func() { fmt.Println(usage) }
	ref, err := term.ParseFileArgs(fs, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return "", false, err
		}
		return "", false, fmt.Errorf("%s needs one ticket or doc (see mdv %s --help)", name, name)
	}
	return ref, *asJSON, nil
}

func RunSignals(dir string, args []string) error {
	ref, asJSON, err := parseRefArgs("signals", signalsUsage, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	all := search.LoadDocs(dir)
	d, err := resolveOne(all, ref)
	if err != nil {
		return err
	}
	r := Signals(all, d, time.Now())
	if asJSON {
		return term.PrintJSON(r)
	}
	printSignals(r)
	return nil
}

func sinceDate(m docs.Meta, now time.Time) (time.Time, string) {
	for _, c := range []struct{ from, value string }{{"merged", m.Merged}, {"updated", m.Updated}, {"created", m.Created}} {
		if t, err := time.Parse("2006-01-02", strings.TrimSpace(c.value)); err == nil {
			return t, c.from
		}
	}
	y, mo, day := now.AddDate(0, 0, -90).Date()
	return time.Date(y, mo, day, 0, 0, 0, 0, now.Location()), "default"
}

// Signals collects warning signs for one doc. It only reads: the docs, the
// local git repo and, when gh is installed, GitHub.
func Signals(all []search.Doc, d search.Doc, now time.Time) SignalsReport {
	m := d.Meta
	since, from := sinceDate(m, now)
	r := SignalsReport{Doc: d.Path, Ticket: m.Ticket, Since: since.Format("2006-01-02"), SinceFrom: from, Warnings: []Signal{}, NotChecked: []NotChecked{}, Note: signalsNote}
	warn := func(kind, ref, format string, a ...any) {
		r.Warnings = append(r.Warnings, Signal{Kind: kind, Ref: ref, Message: fmt.Sprintf(format, a...)})
	}
	skip := func(what, reason string) { r.NotChecked = append(r.NotChecked, NotChecked{what, reason}) }

	r.Checked = append(r.Checked, "recorded issues")
	if m.Status == "regressed" {
		warn("status", m.Ticket, "status is regressed")
	}
	for _, i := range m.OpenIssues() {
		msg := "open issue " + i.Ticket
		if i.Found != "" {
			msg += " (found " + i.Found + ")"
		}
		if i.Note != "" {
			msg += ": " + strings.Join(strings.Fields(i.Note), " ")
		}
		warn("issue", i.Ticket, "%s", strings.TrimSpace(msg))
	}

	repoDir := ""
	switch reposDir, err := ReposDir(); {
	case m.Repo == "":
		skip("git", "no repo in the agent index")
	case err != nil:
		skip("git", err.Error())
	default:
		repoDir = filepath.Join(reposDir, m.Repo)
		if fi, err := os.Stat(repoDir); err != nil || !fi.IsDir() {
			skip("git", "repo not found locally ("+term.Tildify(repoDir)+"), git not checked")
			repoDir = ""
		}
	}
	if repoDir != "" {
		gitSignals(&r, repoDir, m, since, warn, skip)
	}

	githubSignals(&r, repoDir, m, warn, skip)

	if len(m.BuildsOn) > 0 {
		r.Checked = append(r.Checked, "builds_on")
		for _, u := range search.RegressedUpstream(all, d) {
			warn("upstream", search.Label(u.Doc), "%s", u.Message)
		}
		for _, ref := range search.Unresolved(all, m) {
			skip("builds_on "+ref, "no doc found for it in the docs folder")
		}
	}
	return r
}

type commit struct{ hash, date, subject string }

func gitLog(repoDir string, args ...string) ([]commit, error) {
	base := []string{"log", "--date=short", "--format=%H%x1f%ad%x1f%s%x1e"}
	out, err := gitx.Git(repoDir, append(base, args...)...)
	if err != nil {
		return nil, err
	}
	var commits []commit
	for _, rec := range strings.Split(string(out), "\x1e") {
		parts := strings.Split(strings.TrimSpace(rec), "\x1f")
		if len(parts) == 3 {
			commits = append(commits, commit{parts[0], parts[1], parts[2]})
		}
	}
	return commits, nil
}

func gitSignals(r *SignalsReport, repoDir string, m docs.Meta, since time.Time, warn func(kind, ref, format string, a ...any), skip func(what, reason string)) {
	sinceArg := "--since=" + since.Format("2006-01-02") + " 00:00:00"
	seen := map[string]bool{}
	report := func(c commit) {
		if seen[c.hash] {
			return
		}
		seen[c.hash] = true
		warn("commit", c.hash[:min(len(c.hash), 7)], "%s %s  %s", c.hash[:min(len(c.hash), 7)], c.date, c.subject)
	}
	ran := false
	if len(m.Paths) > 0 {
		commits, err := gitLog(repoDir, append([]string{sinceArg, "--"}, m.Paths...)...)
		if err != nil {
			skip("git", err.Error())
			return
		}
		ran = true
		for _, c := range commits {
			if fixSubject.MatchString(c.subject) {
				report(c)
			}
		}
	} else {
		skip("git paths", "no paths in the agent index, so only ticket-based git checks ran")
	}
	if m.Ticket != "" {
		commits, err := gitLog(repoDir, sinceArg, "-i", "--all-match", "--fixed-strings", "--grep="+m.Ticket, "--grep=revert")
		if err != nil {
			skip("git", err.Error())
			return
		}
		ran = true
		for _, c := range commits {
			report(c)
		}
	} else if len(m.Paths) == 0 {
		skip("git", "no paths and no ticket in the agent index")
	}
	if ran {
		r.Checked = append(r.Checked, "git")
	}
}

func githubSignals(r *SignalsReport, repoDir string, m docs.Meta, warn func(kind, ref, format string, a ...any), skip func(what, reason string)) {
	pr := string(m.PR)
	switch {
	case pr == "":
		skip("GitHub", "no pr in the agent index")
		return
	case repoDir == "" && !strings.Contains(pr, "://"):
		skip("GitHub", "repo not found locally, so gh can't tell which repo PR "+pr+" is in")
		return
	}
	if _, err := lookPath("gh"); err != nil {
		skip("GitHub", "gh not found")
		return
	}
	out, err := runGH(repoDir, "pr", "view", pr, "--json", "number,state,title,mergedAt")
	if err != nil {
		skip("GitHub", err.Error())
		return
	}
	var view struct {
		Number int    `json:"number"`
		State  string `json:"state"`
		Title  string `json:"title"`
	}
	if err := json.Unmarshal(out, &view); err != nil {
		skip("GitHub", "gh pr view: "+err.Error())
		return
	}
	out, err = runGH(repoDir, "pr", "list", "--state", "all", "--search", "Revert in:title "+view.Title, "--json", "number,title,state")
	if err != nil {
		skip("GitHub", err.Error())
		return
	}
	var reverts []struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		State  string `json:"state"`
	}
	if err := json.Unmarshal(out, &reverts); err != nil {
		skip("GitHub", "gh pr list: "+err.Error())
		return
	}
	r.Checked = append(r.Checked, "GitHub")
	for _, p := range reverts {
		if p.Number == view.Number {
			continue
		}
		warn("pr", "#"+strconv.Itoa(p.Number), "PR #%d (%s): %s", p.Number, strings.ToLower(p.State), p.Title)
	}
}

func printSignals(r SignalsReport) {
	head := r.Doc
	if r.Ticket != "" {
		head += " (" + r.Ticket + ")"
	}
	since := "since " + r.Since
	if r.SinceFrom != "default" {
		since += " (" + r.SinceFrom + ")"
	} else {
		since += " (last 90 days: no merged, updated or created date in the agent index)"
	}
	fmt.Println(head + term.DimStyle.Render("  "+since))
	if len(r.Warnings) > 0 {
		fmt.Printf("⚠ %d warning sign(s)\n", len(r.Warnings))
		for _, w := range r.Warnings {
			fmt.Printf("  %s %s\n", term.DimStyle.Render(fmt.Sprintf("%-8s", w.Kind)), w.Message)
		}
	} else {
		fmt.Println("✓ no warning signs found in: " + strings.Join(r.Checked, ", "))
	}
	if len(r.NotChecked) > 0 {
		fmt.Println("not checked:")
		for _, n := range r.NotChecked {
			fmt.Printf("  %s: %s\n", n.What, n.Reason)
		}
	}
	fmt.Println(r.Note)
}
