package docs

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/jakobluthman95/mdv/internal/term"
	"gopkg.in/yaml.v3"
)

type Meta struct {
	Ticket       string   `yaml:"ticket" json:"ticket,omitempty"`
	Repo         string   `yaml:"repo" json:"repo,omitempty"`
	Apps         []string `yaml:"apps" json:"apps,omitempty"`
	Type         string   `yaml:"type" json:"type,omitempty"`
	Status       string   `yaml:"status" json:"status,omitempty"`
	Summary      string   `yaml:"summary" json:"summary,omitempty"`
	Keywords     []string `yaml:"keywords" json:"keywords,omitempty"`
	Paths        []string `yaml:"paths" json:"paths,omitempty"`
	Branch       string   `yaml:"branch" json:"branch,omitempty"`
	PR           PRRef    `yaml:"pr" json:"pr,omitempty"`
	Caveats      []string `yaml:"caveats" json:"caveats,omitempty"`
	Related      []string `yaml:"related" json:"related,omitempty"`
	SupersededBy string   `yaml:"superseded_by" json:"supersededBy,omitempty"`
	Merged       string   `yaml:"merged" json:"merged,omitempty"`
	BuildsOn     []string `yaml:"builds_on" json:"buildsOn,omitempty"`
	Verification []string `yaml:"verification" json:"verification,omitempty"`
	Issues       []Issue  `yaml:"issues" json:"issues,omitempty"`
	Created      string   `yaml:"created" json:"created,omitempty"`
	Updated      string   `yaml:"updated" json:"updated,omitempty"`
}

type Issue struct {
	Ticket  string `yaml:"ticket" json:"ticket,omitempty"`
	Found   string `yaml:"found" json:"found,omitempty"`
	Note    string `yaml:"note" json:"note,omitempty"`
	FixedBy string `yaml:"fixed_by" json:"fixedBy,omitempty"`
}

func (m Meta) OpenIssues() []Issue {
	var open []Issue
	for _, i := range m.Issues {
		if strings.TrimSpace(i.FixedBy) == "" {
			open = append(open, i)
		}
	}
	return open
}

func (m Meta) OpenIssueTickets() []string {
	var tickets []string
	for _, i := range m.OpenIssues() {
		if i.Ticket != "" {
			tickets = append(tickets, i.Ticket)
		}
	}
	return tickets
}

// RegressionWarning describes a regressed status or open issues, or is empty.
func (m Meta) RegressionWarning() string {
	what := ""
	switch {
	case m.Status == "regressed":
		what = "regressed"
	case len(m.OpenIssues()) > 0:
		what = "open issues"
	default:
		return ""
	}
	if tickets := m.OpenIssueTickets(); len(tickets) > 0 {
		what += " (" + strings.Join(tickets, ", ") + ")"
	}
	return what
}

type PRRef string

func (p *PRRef) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.ScalarNode {
		return fmt.Errorf("pr must be a number or a URL")
	}
	if n.Tag == "!!null" {
		*p = ""
		return nil
	}
	*p = PRRef(strings.TrimPrefix(n.Value, "#"))
	return nil
}

func (p PRRef) String() string {
	if p == "" {
		return ""
	}
	if _, err := strconv.Atoi(string(p)); err == nil {
		return "PR #" + string(p)
	}
	return string(p)
}

func SplitFrontmatter(content []byte) (meta Meta, body []byte, ok bool, err error) {
	text := bytes.ReplaceAll(content, []byte("\r\n"), []byte("\n"))
	if !bytes.HasPrefix(text, []byte("---\n")) {
		return meta, content, false, nil
	}
	rest := text[4:]
	end := -1
	if bytes.HasPrefix(rest, []byte("---\n")) || bytes.Equal(rest, []byte("---")) {
		end = 0
	} else if i := bytes.Index(rest, []byte("\n---\n")); i >= 0 {
		end = i + 1
	} else if bytes.HasSuffix(rest, []byte("\n---")) {
		end = len(rest) - 3
	}
	if end < 0 {
		return meta, content, false, nil
	}
	block := rest[:end]
	after := rest[min(end+4, len(rest)):]
	body = bytes.TrimLeft(after, "\n")

	var probe map[string]any
	if e := yaml.Unmarshal(block, &probe); e != nil || len(probe) == 0 {
		if !looksLikeYAMLKeys(block) {
			return Meta{}, content, false, nil
		}
		return meta, body, true, fmt.Errorf("agent index is not valid YAML: %v", e)
	}
	if e := yaml.Unmarshal(block, &meta); e != nil {
		return meta, body, true, fmt.Errorf("agent index: %w", e)
	}
	return meta, body, true, nil
}

func StripFrontmatter(content []byte) []byte {
	_, body, _, _ := SplitFrontmatter(content)
	return body
}

var yamlKeyLine = regexp.MustCompile(`^[A-Za-z_][\w-]*:(\s|$)`)

func looksLikeYAMLKeys(block []byte) bool {
	for _, line := range strings.Split(string(block), "\n") {
		if t := strings.TrimSpace(line); t != "" && !strings.HasPrefix(t, "#") {
			return yamlKeyLine.MatchString(line)
		}
	}
	return false
}

func ReadDocBody(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	_, body, _, ferr := SplitFrontmatter(b)
	if ferr != nil {
		fmt.Fprintf(os.Stderr, "mdv: %s: %v\n", term.Tildify(path), ferr)
	}
	return body, nil
}
