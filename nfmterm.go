package main

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/charmbracelet/glamour"
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

func renderForTerminal(md, style string, width int) (string, error) {
	converted := string(nfmForTerminal([]byte(md)))
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
	return nfmPostRender(out), nil
}
