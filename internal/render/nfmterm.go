package render

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/jakobluthman95/mdv/internal/docs"
)

const (
	nfmBlockStart      = "\uE010"
	nfmBlockEnd        = "\uE011"
	nfmStyleEnd   rune = '\uE0FF'
	nfmUnderline  rune = '\uE140'
	nfmFgBase     rune = 0xE100
	nfmBgBase     rune = 0xE120
	nfmColorCount rune = 9
)

var nfmColors = []string{"gray", "brown", "orange", "yellow", "green", "blue", "purple", "pink", "red"}

var (
	nfmFg256 = []int{245, 137, 208, 220, 71, 39, 141, 205, 203}
	nfmBg256 = []int{238, 94, 130, 136, 22, 24, 54, 89, 52}
)

func nfmStyled(color, text string) string {
	if text == "" {
		return text
	}
	bg := strings.HasSuffix(color, "_bg")
	name := strings.TrimSuffix(color, "_bg")
	for i, c := range nfmColors {
		if c == name {
			base := nfmFgBase
			if bg {
				base = nfmBgBase
			}
			return string(base+rune(i)) + text + string(nfmStyleEnd)
		}
	}
	return text
}

func nfmUnderlined(text string) string {
	return string(nfmUnderline) + text + string(nfmStyleEnd)
}

func isNFMStyleRune(r rune) bool {
	return r == nfmStyleEnd || r == nfmUnderline ||
		(r >= nfmFgBase && r < nfmFgBase+nfmColorCount) ||
		(r >= nfmBgBase && r < nfmBgBase+nfmColorCount)
}

func stripNFMStyles(s string) string {
	return strings.Map(func(r rune) rune {
		if isNFMStyleRune(r) {
			return -1
		}
		return r
	}, s)
}

var ansiSeq = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

const nfmGuide = "\x1b[38;5;240m┊\x1b[0m "

func nfmPostRender(rendered string) string {
	var out []string
	depth := 0
	dropBlank := false
	for _, line := range strings.Split(rendered, "\n") {
		plain := strings.TrimSpace(ansiSeq.ReplaceAllString(line, ""))
		switch {
		case strings.Contains(plain, nfmBlockStart):
			depth++
			dropBlank = true
			continue
		case strings.Contains(plain, nfmBlockEnd):
			if n := len(out); n > 0 && strings.Trim(ansiSeq.ReplaceAllString(out[n-1], ""), " ┊") == "" {
				out = out[:n-1]
			}
			depth = max(depth-1, 0)
			dropBlank = true
			continue
		case dropBlank && plain == "":
			dropBlank = false
			continue
		}
		dropBlank = false
		if depth > 0 {
			line = insertAtColumn(line, 2, strings.Repeat(nfmGuide, depth))
		}
		out = append(out, nfmStylesToANSI(line))
	}
	return strings.Join(out, "\n")
}

func insertAtColumn(line string, col int, s string) string {
	visible := 0
	for i := 0; i < len(line); {
		if visible == col {
			return line[:i] + s + line[i:]
		}
		if loc := ansiSeq.FindStringIndex(line[i:]); loc != nil && loc[0] == 0 {
			i += loc[1]
			continue
		}
		visible++
		i++
	}
	return line + strings.Repeat(" ", max(col-visible, 0)) + s
}

func nfmStylesToANSI(s string) string {
	if !strings.ContainsFunc(s, isNFMStyleRune) {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r == nfmStyleEnd:
			b.WriteString("\x1b[39;49;24m")
		case r == nfmUnderline:
			b.WriteString("\x1b[4m")
		case r >= nfmFgBase && r < nfmFgBase+nfmColorCount:
			b.WriteString("\x1b[38;5;" + strconv.Itoa(nfmFg256[r-nfmFgBase]) + "m")
		case r >= nfmBgBase && r < nfmBgBase+nfmColorCount:
			b.WriteString("\x1b[48;5;" + strconv.Itoa(nfmBg256[r-nfmBgBase]) + "m")
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ForTerminal renders a doc for the viewer. warnings are extra regression
// warnings for the agent-index header, e.g. dependencies that regressed.
func ForTerminal(md, style string, width int, warnings ...string) (string, error) {
	meta, body, hasMeta, metaErr := docs.SplitFrontmatter([]byte(md))
	converted := string(nfmForTerminal(body))
	if strings.Contains(converted, nfmBlockStart) {
		width -= 4
	}
	r, err := glamour.NewTermRenderer(glamour.WithStyles(markdownStyle(style)), glamour.WithWordWrap(max(width, 20)))
	if err != nil {
		return "", err
	}
	out, err := r.Render(converted)
	if err != nil {
		return "", err
	}
	header := ""
	switch {
	case metaErr != nil:
		header = agentIndexError(metaErr)
	case hasMeta:
		header = agentIndexHeader(meta, max(width, 20), warnings)
	}
	return header + nfmPostRender(out), nil
}

var (
	indexTitleStyle = lipgloss.NewStyle().Bold(true)
	indexDimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	indexWarnStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	indexBoxStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240")).Padding(0, 1).MarginLeft(2)
)

func agentIndexHeader(m docs.Meta, width int, warnings []string) string {
	var parts []string
	for _, s := range []string{m.Ticket, strings.Trim(m.Repo+"/"+strings.Join(m.Apps, ","), "/"), m.Type, m.Status, m.PR.String()} {
		if s != "" {
			parts = append(parts, s)
		}
	}
	inner := max(width-8, 20)
	wrap := lipgloss.NewStyle().Width(inner)
	lines := []string{indexTitleStyle.Render("🤖 " + strings.Join(parts, " · "))}
	warn := indexWarnStyle.Inherit(wrap)
	label := "open issue"
	if m.Status == "regressed" {
		label = "regressed"
		if len(m.OpenIssues()) == 0 {
			lines = append(lines, warn.Render("⚠ regressed"))
		}
	}
	for _, i := range m.OpenIssues() {
		lines = append(lines, warn.Render(strings.TrimSpace("⚠ "+label+": "+i.Ticket+" "+strings.Join(strings.Fields(i.Note), " "))))
	}
	for _, w := range warnings {
		lines = append(lines, warn.Render("⚠ "+w))
	}
	if s := strings.TrimSpace(m.Summary); s != "" {
		lines = append(lines, wrap.Render(s))
	}
	if len(m.Keywords) > 0 {
		lines = append(lines, indexDimStyle.Inherit(wrap).Render("keywords: "+strings.Join(m.Keywords, ", ")))
	}
	for _, c := range m.Caveats {
		lines = append(lines, indexDimStyle.Inherit(wrap).Render("⚠ "+c))
	}
	if m.Updated != "" {
		lines = append(lines, indexDimStyle.Render("updated "+m.Updated))
	}
	return "\n" + indexBoxStyle.Render(strings.Join(lines, "\n")) + "\n"
}

func agentIndexError(err error) string {
	return "\n" + indexBoxStyle.Render(indexDimStyle.Render("⚠ "+err.Error())) + "\n"
}
