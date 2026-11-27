package search

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jakobluthman95/mdv/internal/docs"
)

var ticketRef = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]+-\d+$`)

// Resolve finds the docs a builds_on or related entry points at: a ticket
// matches a doc's ticket, a path matches the doc's path in the docs folder,
// with or without .md.
func Resolve(all []Doc, ref string) []Doc {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil
	}
	var out []Doc
	if ticketRef.MatchString(ref) {
		for _, d := range all {
			if strings.EqualFold(d.Meta.Ticket, ref) {
				out = append(out, d)
			}
		}
		return out
	}
	want := normalizeRef(ref)
	for _, d := range all {
		if normalizeRef(d.Path) == want || normalizeRef(d.AbsPath) == want {
			out = append(out, d)
		}
	}
	return out
}

func normalizeRef(p string) string {
	if expanded, err := docs.ExpandPath(p); err == nil && (filepath.IsAbs(p) || strings.HasPrefix(p, "~")) {
		p = expanded
	}
	p = strings.TrimPrefix(filepath.ToSlash(p), "./")
	if docs.IsMarkdown(p) {
		p = strings.TrimSuffix(p, filepath.Ext(p))
	}
	return strings.ToLower(p)
}

// Label names a doc by its ticket, or by its path when it has none.
func Label(d Doc) string {
	if d.Meta.Ticket != "" {
		return d.Meta.Ticket
	}
	return d.Path
}

// Unresolved lists the builds_on entries that match no doc.
func Unresolved(all []Doc, m docs.Meta) []string {
	var out []string
	for _, ref := range m.BuildsOn {
		if len(Resolve(all, ref)) == 0 {
			out = append(out, ref)
		}
	}
	return out
}

type Upstream struct {
	Doc     Doc
	Via     []string
	Message string
}

// RegressedUpstream walks builds_on transitively and returns every
// dependency that regressed or has open issues.
func RegressedUpstream(all []Doc, self Doc) []Upstream {
	var out []Upstream
	seen := map[string]bool{self.AbsPath: true}
	var walk func(m docs.Meta, via []string)
	walk = func(m docs.Meta, via []string) {
		for _, ref := range m.BuildsOn {
			for _, dep := range Resolve(all, ref) {
				if seen[dep.AbsPath] {
					continue
				}
				seen[dep.AbsPath] = true
				if dep.Meta.RegressionWarning() != "" {
					msg := "builds on " + Label(dep)
					if len(via) > 0 {
						msg += " (via " + strings.Join(via, " → ") + ")"
					}
					if dep.Meta.Status == "regressed" {
						msg += ", which regressed"
					} else {
						msg += ", which has open issues"
					}
					if tickets := dep.Meta.OpenIssueTickets(); len(tickets) > 0 {
						msg += " (" + strings.Join(tickets, ", ") + ")"
					}
					out = append(out, Upstream{Doc: dep, Via: append([]string(nil), via...), Message: msg})
				}
				walk(dep.Meta, append(via, Label(dep)))
			}
		}
	}
	walk(self.Meta, nil)
	return out
}

// Warnings are the regression warnings for one doc: its own status and
// open issues, then regressed dependencies.
func Warnings(all []Doc, d Doc) []string {
	out := []string{}
	if w := d.Meta.RegressionWarning(); w != "" {
		out = append(out, w)
	}
	for _, u := range RegressedUpstream(all, d) {
		out = append(out, u.Message)
	}
	return out
}
