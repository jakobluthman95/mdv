package safeguard

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jakobluthman95/mdv/internal/search"
)

var testNow = time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC)

func writeDocs(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func git(t *testing.T, dir string, env []string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func commitFile(t *testing.T, repo, rel, date, subject string) {
	t.Helper()
	p := filepath.Join(repo, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(subject + "\n")
	f.Close()
	git(t, repo, nil, "add", "-A")
	stamp := date + "T12:00:00"
	git(t, repo, []string{"GIT_AUTHOR_DATE=" + stamp, "GIT_COMMITTER_DATE=" + stamp}, "commit", "-q", "-m", subject)
}

func initRepo(t *testing.T, reposDir, name string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	repo := filepath.Join(reposDir, name)
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, repo, nil, "init", "-q")
	git(t, repo, nil, "config", "user.name", "Test")
	git(t, repo, nil, "config", "user.email", "test@example.com")
	git(t, repo, nil, "config", "commit.gpgsign", "false")
	return repo
}

func noGH(t *testing.T) {
	t.Helper()
	old := lookPath
	lookPath = func(string) (string, error) { return "", errors.New("not found") }
	t.Cleanup(func() { lookPath = old })
}

const httpDoc = "---\nticket: TAP-222\nrepo: shop\ntype: summary\nstatus: done\nsummary: Replaced axios with fetch in the http package.\nkeywords: [axios]\npaths: [packages/http]\nmerged: 2026-11-01\nupdated: 2026-11-02\n---\n# Remove axios\n"

func signalsFor(t *testing.T, dir, ref string) SignalsReport {
	t.Helper()
	all := search.LoadDocs(dir)
	d, err := resolveOne(all, ref)
	if err != nil {
		t.Fatal(err)
	}
	return Signals(all, d, testNow)
}

func messages(r SignalsReport) string {
	var b strings.Builder
	for _, w := range r.Warnings {
		b.WriteString(w.Kind + ": " + w.Message + "\n")
	}
	return b.String()
}

func TestSignalsGit(t *testing.T) {
	noGH(t)
	reposDir := t.TempDir()
	t.Setenv("MDV_REPOS_DIR", reposDir)
	repo := initRepo(t, reposDir, "shop")
	commitFile(t, repo, "packages/http/client.ts", "2026-10-20", "fix: old fix before the merge")
	commitFile(t, repo, "packages/http/client.ts", "2026-11-10", "fix: header dropped by fetch client")
	commitFile(t, repo, "packages/http/client.ts", "2026-11-11", "Refactor client options")
	commitFile(t, repo, "packages/ui/button.ts", "2026-11-12", "fix: button colour")
	commitFile(t, repo, "README.md", "2026-11-13", "Revert \"TAP-222 remove axios\"")
	dir := writeDocs(t, map[string]string{"shop/TAP-222-remove-axios.md": httpDoc})

	r := signalsFor(t, dir, "TAP-222")
	got := messages(r)
	for _, want := range []string{"header dropped by fetch client", "Revert \"TAP-222 remove axios\""} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"button colour", "Refactor client options", "old fix before the merge"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("unexpected %q in:\n%s", unwanted, got)
		}
	}
	if len(r.Warnings) != 2 || r.Since != "2026-11-01" || r.SinceFrom != "merged" {
		t.Errorf("report: %+v", r)
	}
	if r.Note == "" || !strings.Contains(r.Note, "Jira and Sentry") {
		t.Errorf("note: %q", r.Note)
	}
}

func TestSignalsCleanHistory(t *testing.T) {
	noGH(t)
	reposDir := t.TempDir()
	t.Setenv("MDV_REPOS_DIR", reposDir)
	repo := initRepo(t, reposDir, "shop")
	commitFile(t, repo, "packages/http/client.ts", "2026-11-10", "Add retries to the client")
	dir := writeDocs(t, map[string]string{"shop/TAP-222-remove-axios.md": httpDoc})

	r := signalsFor(t, dir, "shop/TAP-222-remove-axios")
	if len(r.Warnings) != 0 {
		t.Fatalf("clean history: %s", messages(r))
	}
	out := captureStdout(t, func() { printSignals(r) })
	if !strings.Contains(out, "✓ no warning signs found in: recorded issues, git\n") || !strings.Contains(out, "GitHub: no pr in the agent index") {
		t.Errorf("verdict:\n%s", out)
	}
	if !strings.HasSuffix(out, signalsNote+"\n") {
		t.Errorf("must end with the Jira/Sentry note:\n%s", out)
	}
}

func TestSignalsMissingRepoAndNoPaths(t *testing.T) {
	noGH(t)
	t.Setenv("MDV_REPOS_DIR", t.TempDir())
	dir := writeDocs(t, map[string]string{"shop/TAP-222-remove-axios.md": httpDoc})
	r := signalsFor(t, dir, "TAP-222")
	missing := false
	for _, n := range r.NotChecked {
		missing = missing || (n.What == "git" && strings.Contains(n.Reason, "repo not found locally"))
	}
	for _, c := range r.Checked {
		if c == "git" {
			t.Errorf("git can't be checked without the repo: %+v", r)
		}
	}
	if !missing {
		t.Errorf("missing repo should be under not checked: %+v", r.NotChecked)
	}

	reposDir := t.TempDir()
	t.Setenv("MDV_REPOS_DIR", reposDir)
	initRepo(t, reposDir, "shop")
	commitFile(t, filepath.Join(reposDir, "shop"), "a.txt", "2026-11-10", "fix: something unrelated")
	dir = writeDocs(t, map[string]string{"shop/TAP-7-notes.md": "# Notes without an agent index\n"})
	r = signalsFor(t, dir, "TAP-7")
	noPaths := false
	for _, n := range r.NotChecked {
		noPaths = noPaths || strings.Contains(n.Reason, "no paths in the agent index, so only ticket-based git checks ran")
	}
	if !noPaths || len(r.Warnings) != 0 || r.SinceFrom != "default" {
		t.Errorf("doc without paths: %+v", r)
	}
}

func TestSignalsRecordedAndUpstream(t *testing.T) {
	noGH(t)
	t.Setenv("MDV_REPOS_DIR", t.TempDir())
	dir := writeDocs(t, map[string]string{
		"shop/TAP-222-remove-axios.md": strings.Replace(httpDoc, "status: done", "status: regressed\nissues:\n  - ticket: TAP-555\n    found: 2026-11-20\n    note: headers dropped\n  - ticket: TAP-556\n    found: 2026-11-21\n    fixed_by: TAP-557", 1),
		"shop/TAP-333-retries.md":      "---\nticket: TAP-333\nrepo: shop\nbuilds_on: [TAP-222]\n---\n# Retries\n",
	})
	r := signalsFor(t, dir, "TAP-222")
	got := messages(r)
	if !strings.Contains(got, "status: status is regressed") || !strings.Contains(got, "open issue TAP-555 (found 2026-11-20): headers dropped") || strings.Contains(got, "TAP-556") {
		t.Errorf("recorded:\n%s", got)
	}
	r = signalsFor(t, dir, "TAP-333")
	if got := messages(r); !strings.Contains(got, "upstream: builds on TAP-222, which regressed (TAP-555)") {
		t.Errorf("upstream:\n%s", got)
	}
}

func TestSignalsGitHubInjected(t *testing.T) {
	reposDir := t.TempDir()
	t.Setenv("MDV_REPOS_DIR", reposDir)
	initRepo(t, reposDir, "shop")
	oldLook, oldRun := lookPath, runGH
	t.Cleanup(func() { lookPath, runGH = oldLook, oldRun })
	lookPath = func(string) (string, error) { return "/bin/gh", nil }
	var calls []string
	runGH = func(dir string, args ...string) ([]byte, error) {
		calls = append(calls, strings.Join(args, " "))
		if args[1] == "view" {
			return []byte(`{"number":42,"state":"MERGED","title":"Remove axios","mergedAt":"2026-11-01T10:00:00Z"}`), nil
		}
		return []byte(`[{"number":50,"title":"Revert \"Remove axios\"","state":"MERGED"}]`), nil
	}
	dir := writeDocs(t, map[string]string{"shop/TAP-222-remove-axios.md": strings.Replace(httpDoc, "merged:", "pr: 42\nmerged:", 1)})
	r := signalsFor(t, dir, "TAP-222")
	if got := messages(r); !strings.Contains(got, `pr: PR #50 (merged): Revert "Remove axios"`) {
		t.Errorf("revert PR:\n%s", got)
	}
	if len(calls) != 2 || !strings.Contains(calls[1], "Revert in:title Remove axios") {
		t.Errorf("gh calls: %v", calls)
	}
}

func TestSignalsAmbiguousRef(t *testing.T) {
	dir := writeDocs(t, map[string]string{
		"shop/TAP-1-plan.md":    "---\nticket: TAP-1\nrepo: shop\n---\n# Plan\n",
		"shop/TAP-1-summary.md": "---\nticket: TAP-1\nrepo: shop\n---\n# Summary\n",
	})
	err := RunSignals(dir, []string{"TAP-1"})
	if err == nil || !strings.Contains(err.Error(), "pass a path") || !strings.Contains(err.Error(), "shop/TAP-1-plan.md") {
		t.Fatalf("ambiguous: %v", err)
	}
}

func TestImpact(t *testing.T) {
	dir := writeDocs(t, map[string]string{
		"shop/TAP-1-a.md":       "---\nticket: TAP-1\nrepo: shop\nstatus: done\nbuilds_on: [shop/TAP-3-c.md]\n---\n# A\n",
		"shop/TAP-2-b.md":       "---\nticket: TAP-2\nrepo: shop\nstatus: done\nbuilds_on: [TAP-1]\n---\n# B\n",
		"shop/TAP-3-c.md":       "---\nticket: TAP-3\nrepo: shop\nstatus: in-progress\nbuilds_on: [shop/TAP-2-b]\n---\n# C\n",
		"shop/TAP-4-related.md": "---\nticket: TAP-4\nrepo: shop\nrelated: [TAP-1]\n---\n# D\n",
		"shop/TAP-5-other.md":   "---\nticket: TAP-5\nrepo: shop\n---\n# E\n",
	})
	all := search.LoadDocs(dir)
	done := make(chan ImpactReport)
	go func() { done <- Impact(all, search.Resolve(all, "TAP-1")) }()
	var r ImpactReport
	select {
	case r = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("impact did not finish on a cycle")
	}
	if len(r.Dependents) != 2 || r.Dependents[0].Path != "shop/TAP-2-b.md" || r.Dependents[0].Depth != 1 || r.Dependents[1].Path != "shop/TAP-3-c.md" || r.Dependents[1].Depth != 2 || r.Dependents[1].Via != "shop/TAP-2-b.md" {
		t.Fatalf("dependents: %+v", r.Dependents)
	}
	if len(r.Related) != 1 || r.Related[0].Ticket != "TAP-4" {
		t.Fatalf("related: %+v", r.Related)
	}
	out := captureStdout(t, func() { printImpact(r) })
	if !strings.Contains(out, "  └ shop/TAP-2-b.md") || !strings.Contains(out, "    └ shop/TAP-3-c.md") || !strings.Contains(out, "may be affected (lists it under related):\n  shop/TAP-4-related.md") {
		t.Errorf("tree:\n%s", out)
	}
	if up := search.RegressedUpstream(all, search.Resolve(all, "TAP-3")[0]); len(up) != 0 {
		t.Errorf("cycle without regressions: %+v", up)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	b, _ := io.ReadAll(r)
	return string(b)
}
