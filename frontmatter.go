package main

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type docMeta struct {
	Ticket       string   `yaml:"ticket" json:"ticket,omitempty"`
	Repo         string   `yaml:"repo" json:"repo,omitempty"`
	Apps         []string `yaml:"apps" json:"apps,omitempty"`
	Type         string   `yaml:"type" json:"type,omitempty"`
	Status       string   `yaml:"status" json:"status,omitempty"`
	Summary      string   `yaml:"summary" json:"summary,omitempty"`
	Keywords     []string `yaml:"keywords" json:"keywords,omitempty"`
	Paths        []string `yaml:"paths" json:"paths,omitempty"`
	Branch       string   `yaml:"branch" json:"branch,omitempty"`
	PR           prRef    `yaml:"pr" json:"pr,omitempty"`
	Caveats      []string `yaml:"caveats" json:"caveats,omitempty"`
	Related      []string `yaml:"related" json:"related,omitempty"`
	SupersededBy string   `yaml:"superseded_by" json:"supersededBy,omitempty"`
	Created      string   `yaml:"created" json:"created,omitempty"`
	Updated      string   `yaml:"updated" json:"updated,omitempty"`
}

type prRef string

func (p *prRef) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind != yaml.ScalarNode {
		return fmt.Errorf("pr must be a number or a URL")
	}
	if n.Tag == "!!null" {
		*p = ""
		return nil
	}
	*p = prRef(strings.TrimPrefix(n.Value, "#"))
	return nil
}

func (p prRef) String() string {
	if p == "" {
		return ""
	}
	if _, err := strconv.Atoi(string(p)); err == nil {
		return "PR #" + string(p)
	}
	return string(p)
}

func splitFrontmatter(content []byte) (meta docMeta, body []byte, ok bool, err error) {
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
			return docMeta{}, content, false, nil
		}
		return meta, body, true, fmt.Errorf("agent index is not valid YAML: %v", e)
	}
	if e := yaml.Unmarshal(block, &meta); e != nil {
		return meta, body, true, fmt.Errorf("agent index: %w", e)
	}
	return meta, body, true, nil
}

func stripFrontmatter(content []byte) []byte {
	_, body, _, _ := splitFrontmatter(content)
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

func readDocBody(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	_, body, _, ferr := splitFrontmatter(b)
	if ferr != nil {
		fmt.Fprintf(os.Stderr, "mdv: %s: %v\n", tildify(path), ferr)
	}
	return body, nil
}
