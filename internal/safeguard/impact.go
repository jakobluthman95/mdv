package safeguard

import (
	"errors"
	"flag"
	"fmt"
	"strings"

	"github.com/jakobluthman95/mdv/internal/search"
	"github.com/jakobluthman95/mdv/internal/term"
)

const impactUsage = `Usage:
  mdv impact <ticket|doc> [--json]   list the docs that build on a doc (read-only)

Shows every doc whose builds_on points at it, directly or through other docs,
as a tree, then the docs that only list it under related. A ticket with
several docs counts as all of them.`

type ImpactDoc struct {
	Path   string `json:"path"`
	Ticket string `json:"ticket,omitempty"`
	Status string `json:"status,omitempty"`
	Depth  int    `json:"depth"`
	Via    string `json:"via,omitempty"`
}

type ImpactReport struct {
	Docs       []string    `json:"docs"`
	Dependents []ImpactDoc `json:"dependents"`
	Related    []ImpactDoc `json:"related"`
}

func impactDoc(d search.Doc, depth int, via string) ImpactDoc {
	return ImpactDoc{Path: d.Path, Ticket: d.Meta.Ticket, Status: d.Meta.Status, Depth: depth, Via: via}
}

// Impact lists the docs that build on targets, breadth first so each doc
// appears once at its shortest distance, then the docs that only relate to them.
func Impact(all []search.Doc, targets []search.Doc) ImpactReport {
	r := ImpactReport{Docs: []string{}, Dependents: []ImpactDoc{}, Related: []ImpactDoc{}}
	seen := map[string]bool{}
	for _, t := range targets {
		r.Docs = append(r.Docs, t.Path)
		seen[t.AbsPath] = true
	}
	resolves := func(refs []string, to search.Doc) bool {
		for _, ref := range refs {
			for _, d := range search.Resolve(all, ref) {
				if d.AbsPath == to.AbsPath {
					return true
				}
			}
		}
		return false
	}

	frontier := targets
	for depth := 1; len(frontier) > 0; depth++ {
		var next []search.Doc
		for _, parent := range frontier {
			for _, d := range all {
				if seen[d.AbsPath] || !resolves(d.Meta.BuildsOn, parent) {
					continue
				}
				seen[d.AbsPath] = true
				r.Dependents = append(r.Dependents, impactDoc(d, depth, parent.Path))
				next = append(next, d)
			}
		}
		frontier = next
	}

	for _, d := range all {
		if seen[d.AbsPath] {
			continue
		}
		for _, t := range targets {
			if resolves(d.Meta.Related, t) {
				seen[d.AbsPath] = true
				r.Related = append(r.Related, impactDoc(d, 1, t.Path))
				break
			}
		}
	}
	return r
}

func RunImpact(dir string, args []string) error {
	ref, asJSON, err := parseRefArgs("impact", impactUsage, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	all := search.LoadDocs(dir)
	targets := search.Resolve(all, ref)
	if len(targets) == 0 {
		return fmt.Errorf("no doc found for %s (pass a ticket or a path in the docs folder)", ref)
	}
	r := Impact(all, targets)
	if asJSON {
		return term.PrintJSON(r)
	}
	printImpact(r)
	return nil
}

func describe(d ImpactDoc) string {
	s := d.Path
	for _, f := range []string{d.Ticket, d.Status} {
		if f != "" {
			s += term.DimStyle.Render(" · " + f)
		}
	}
	return s
}

func printImpact(r ImpactReport) {
	fmt.Println("Impact of " + strings.Join(r.Docs, ", "))
	if len(r.Dependents) == 0 {
		fmt.Println("No docs build on it (builds_on).")
	} else {
		fmt.Println("builds on it:")
		children := map[string][]ImpactDoc{}
		for _, d := range r.Dependents {
			children[d.Via] = append(children[d.Via], d)
		}
		var walk func(parent string, indent int)
		walk = func(parent string, indent int) {
			for _, d := range children[parent] {
				fmt.Println(strings.Repeat("  ", indent) + "└ " + describe(d))
				walk(d.Path, indent+1)
			}
		}
		for _, t := range r.Docs {
			walk(t, 1)
		}
	}
	if len(r.Related) > 0 {
		fmt.Println("may be affected (lists it under related):")
		for _, d := range r.Related {
			fmt.Println("  " + describe(d))
		}
	}
}
