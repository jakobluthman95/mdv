package main

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

type nfmTarget int

const (
	nfmTerminal nfmTarget = iota
	nfmGist
)

func nfmForTerminal(md []byte) []byte { return []byte(renderNFM(string(md), nfmTerminal)) }
func nfmForGist(md []byte) []byte     { return []byte(renderNFM(string(md), nfmGist)) }

var nfmBlockTags = []string{
	"toggle", "callout", "quote", "columns", "column", "tabs", "tab",
	"p", "h1", "h2", "h3", "h4", "ul", "ol", "li", "todo", "equation", "table",
	"image", "file", "pdf", "audio", "video",
}

var nfmInlineTags = []string{
	"span", "b", "i", "s", "u", "code", "br", "date", "mention",
	"bookmark", "embed", "file", "pdf", "audio", "video", "image", "table-of-contents",
}

var (
	nfmOpenLine  = regexp.MustCompile(`^<(` + strings.Join(nfmBlockTags, "|") + `)((?:\s+[a-z-]+="[^"]*")*)\s*>$`)
	nfmCloseLine = regexp.MustCompile(`^</(` + strings.Join(nfmBlockTags, "|") + `)>$`)
	nfmSelfLine  = regexp.MustCompile(`^<(table-of-contents|p)((?:\s+[a-z-]+="[^"]*")*)\s*/>$`)
	nfmAttr      = regexp.MustCompile(`([a-z-]+)="([^"]*)"`)
	nfmFence     = regexp.MustCompile("^\\s*(```+|~~~+)")
)

type nfmFrame struct {
	tag       string
	attrs     map[string]string
	wantTitle bool
	quoted    bool
	listItem  string
	columns   int
}

func nfmAttrs(s string) map[string]string {
	attrs := map[string]string{}
	for _, m := range nfmAttr.FindAllStringSubmatch(s, -1) {
		attrs[m[1]] = html.UnescapeString(m[2])
	}
	return attrs
}

func renderNFM(md string, target nfmTarget) string {
	var out []string
	var stack []*nfmFrame
	var table []string
	fence := ""
	skipBlank := false

	prefix := func() string {
		p := ""
		for _, f := range stack {
			if f.quoted {
				p += "> "
			}
		}
		return p
	}
	emit := func(line string) {
		p := prefix()
		if line == "" {
			out = append(out, strings.TrimRight(p, " "))
			return
		}
		out = append(out, p+line)
	}
	top := func() *nfmFrame {
		if len(stack) == 0 {
			return nil
		}
		return stack[len(stack)-1]
	}

	for _, line := range strings.Split(md, "\n") {
		trimmed := strings.TrimSpace(line)
		if skipBlank {
			skipBlank = false
			if trimmed == "" {
				continue
			}
		}

		if fence != "" {
			emit(line)
			if strings.HasPrefix(trimmed, fence) && strings.Trim(trimmed, fence[:1]) == "" {
				fence = ""
			}
			continue
		}
		if m := nfmFence.FindStringSubmatch(line); m != nil && table == nil {
			fence = m[1]
			emit(line)
			continue
		}

		if table != nil {
			table = append(table, line)
			if trimmed == "</table>" {
				for _, l := range nfmTable(strings.Join(table, "\n"), target) {
					emit(l)
				}
				table = nil
			}
			continue
		}
		if strings.HasPrefix(trimmed, "<table") && nfmOpenLine.MatchString(trimmed) {
			table = []string{line}
			continue
		}

		if m := nfmSelfLine.FindStringSubmatch(trimmed); m != nil {
			if m[1] == "p" {
				emit("")
			}
			continue
		}

		if m := nfmOpenLine.FindStringSubmatch(trimmed); m != nil {
			f := &nfmFrame{tag: m[1], attrs: nfmAttrs(m[2])}
			switch f.tag {
			case "toggle", "callout", "quote", "tab", "p", "h1", "h2", "h3", "h4", "li", "todo",
				"image", "file", "pdf", "audio", "video":
				f.wantTitle = true
			case "equation":
				emit("```math")
			case "column":
				emit("")
				if target == nfmTerminal {
					n := 1
					if t := top(); t != nil && t.tag == "columns" {
						t.columns++
						n = t.columns
					}
					emit(fmt.Sprintf("*◫ Column %d*", n))
					emit("")
					emit(nfmBlockStart)
					emit("")
				}
			}
			if f.tag == "li" {
				f.listItem = "- "
				if t := top(); t != nil && t.tag == "ol" {
					f.listItem = "1. "
				}
			}
			stack = append(stack, f)
			continue
		}

		if m := nfmCloseLine.FindStringSubmatch(trimmed); m != nil && top() != nil && top().tag == m[1] {
			f := top()
			stack = stack[:len(stack)-1]
			switch f.tag {
			case "toggle", "tab", "column":
				emit("")
				if target == nfmGist {
					if f.tag == "toggle" {
						emit("</details>")
					}
				} else {
					emit(nfmBlockEnd)
					emit("")
				}
			case "equation":
				emit("```")
			case "image", "file", "pdf", "audio", "video":
				if f.wantTitle {
					emit(nfmMediaLink(f.tag, f.attrs, "", target))
				}
			}
			continue
		}

		f := top()
		if f != nil && f.wantTitle {
			if trimmed == "" {
				continue
			}
			f.wantTitle = false
			title := renderNFMInline(trimmed, target)
			if target == nfmTerminal {
				title = nfmStyled(f.attrs["color"], title)
			}
			openBlock := func() {
				emit("")
				if target == nfmTerminal {
					emit(nfmBlockStart)
					emit("")
				}
				skipBlank = true
			}
			switch f.tag {
			case "toggle":
				if h := f.attrs["heading"]; h != "" {
					n := int(h[len(h)-1] - '0')
					if target == nfmGist {
						emit(fmt.Sprintf("<details><summary><strong>%s</strong></summary>", title))
					} else {
						emit(strings.Repeat("#", n) + " ▸ " + title)
					}
				} else if target == nfmGist {
					emit("<details><summary>" + title + "</summary>")
				} else {
					emit("▸ **" + title + "**")
				}
				openBlock()
			case "callout", "quote":
				f.quoted = true
				if icon := f.attrs["icon"]; icon != "" && !strings.Contains(icon, "/") {
					title = icon + " " + title
				}
				emit(title)
				emit("")
				skipBlank = true
			case "tab":
				label := title
				if icon := f.attrs["icon"]; icon != "" && !strings.Contains(icon, "/") {
					label = icon + " " + label
				}
				emit("**" + label + "**")
				openBlock()
			case "image", "file", "pdf", "audio", "video":
				emit(nfmMediaLink(f.tag, f.attrs, title, target))
			case "h1", "h2", "h3", "h4":
				emit(strings.Repeat("#", int(f.tag[1]-'0')) + " " + title)
			case "li":
				emit(f.listItem + title)
			case "todo":
				box := "[ ]"
				if f.attrs["checked"] == "true" {
					box = "[x]"
				}
				emit("- " + box + " " + title)
			default:
				emit(title)
			}
			continue
		}

		emit(renderNFMInline(line, target))
	}
	return strings.Join(out, "\n")
}

var (
	nfmSpan      = regexp.MustCompile(`<span(?:\s+[a-z-]+="[^"]*")*\s*>(.*?)</span>`)
	nfmPair      = regexp.MustCompile(`<(b|i|s|u|code)>(.*?)</(?:b|i|s|u|code)>`)
	nfmBr        = regexp.MustCompile(`<br\s*/?>`)
	nfmDate      = regexp.MustCompile(`<date((?:\s+[a-z-]+="[^"]*")*)\s*/>`)
	nfmMention   = regexp.MustCompile(`<mention((?:\s+[a-z-]+="[^"]*")*)\s*(?:/>|>(.*?)</mention>)`)
	nfmMedia     = regexp.MustCompile(`<(bookmark|embed|file|pdf|audio|video|image)((?:\s+[a-z-]+="[^"]*")*)\s*(?:/>|>(.*?)</(?:bookmark|embed|file|pdf|audio|video|image)>)`)
	nfmToc       = regexp.MustCompile(`<table-of-contents(?:\s+[a-z-]+="[^"]*")*\s*/>`)
	nfmCitation  = regexp.MustCompile(`\[\^(https?://[^\]\s]+)\]`)
	nfmCodeSpans = regexp.MustCompile("`+[^`]*`+")
)

func renderNFMInline(line string, target nfmTarget) string {
	spans := nfmCodeSpans.FindAllStringIndex(line, -1)
	var b strings.Builder
	last := 0
	for _, s := range spans {
		b.WriteString(renderNFMInlineText(line[last:s[0]], target))
		b.WriteString(line[s[0]:s[1]])
		last = s[1]
	}
	b.WriteString(renderNFMInlineText(line[last:], target))
	return b.String()
}

func renderNFMInlineText(s string, target nfmTarget) string {
	if !strings.ContainsAny(s, "<[") {
		return s
	}
	s = nfmToc.ReplaceAllString(s, "")
	for i := 0; i < 3; i++ {
		s = nfmSpan.ReplaceAllStringFunc(s, func(m string) string {
			sub := nfmSpan.FindStringSubmatch(m)
			if target == nfmTerminal {
				return nfmStyled(nfmAttrs(m[:strings.Index(m, ">")])["color"], sub[1])
			}
			return sub[1]
		})
	}
	s = nfmPair.ReplaceAllStringFunc(s, func(m string) string {
		sub := nfmPair.FindStringSubmatch(m)
		switch sub[1] {
		case "b":
			return "**" + sub[2] + "**"
		case "i":
			return "*" + sub[2] + "*"
		case "s":
			return "~~" + sub[2] + "~~"
		case "code":
			return "`" + sub[2] + "`"
		}
		if target == nfmGist {
			return "<ins>" + sub[2] + "</ins>"
		}
		return nfmUnderlined(sub[2])
	})
	if target == nfmTerminal {
		s = nfmBr.ReplaceAllString(s, "  \n")
	} else {
		s = nfmBr.ReplaceAllString(s, "<br>")
	}
	s = nfmDate.ReplaceAllStringFunc(s, func(m string) string {
		a := nfmAttrs(nfmDate.FindStringSubmatch(m)[1])
		d := strings.TrimSpace(a["start"] + " " + a["start-time"])
		if a["end"] != "" || a["end-time"] != "" {
			d += " → " + strings.TrimSpace(a["end"]+" "+a["end-time"])
		}
		return d
	})
	s = nfmMention.ReplaceAllStringFunc(s, func(m string) string {
		sub := nfmMention.FindStringSubmatch(m)
		u, label := nfmAttrs(sub[1])["url"], sub[2]
		if strings.HasPrefix(u, "http") {
			if label == "" {
				label = u
			}
			return "[" + label + "](" + u + ")"
		}
		if label != "" {
			return "@" + label
		}
		return u
	})
	s = nfmMedia.ReplaceAllStringFunc(s, func(m string) string {
		sub := nfmMedia.FindStringSubmatch(m)
		return nfmMediaLink(sub[1], nfmAttrs(sub[2]), strings.TrimSpace(sub[3]), target)
	})
	s = nfmCitation.ReplaceAllString(s, "[[source]]($1)")
	return s
}

var (
	nfmRow  = regexp.MustCompile(`(?s)<tr(?:\s+[a-z-]+="[^"]*")*\s*>(.*?)</tr>`)
	nfmCell = regexp.MustCompile(`(?s)<t[hd](?:\s+[a-z-]+="[^"]*")*\s*>(.*?)</t[hd]>`)
)

func nfmTable(block string, target nfmTarget) []string {
	var rows [][]string
	width := 0
	for _, r := range nfmRow.FindAllStringSubmatch(block, -1) {
		var cells []string
		for _, c := range nfmCell.FindAllStringSubmatch(r[1], -1) {
			text := strings.Join(strings.Fields(c[1]), " ")
			text = nfmBr.ReplaceAllString(text, " ")
			text = strings.ReplaceAll(stripNFMStyles(renderNFMInline(text, target)), "|", `\|`)
			cells = append(cells, text)
		}
		width = max(width, len(cells))
		rows = append(rows, cells)
	}
	if len(rows) == 0 {
		return strings.Split(block, "\n")
	}
	line := func(cells []string) string {
		for len(cells) < width {
			cells = append(cells, "")
		}
		return "| " + strings.Join(cells, " | ") + " |"
	}
	out := []string{line(rows[0]), "|" + strings.Repeat(" --- |", width)}
	for _, r := range rows[1:] {
		out = append(out, line(r))
	}
	return out
}

func nfmMediaLink(kind string, a map[string]string, caption string, target nfmTarget) string {
	src := a["source"]
	if src == "" {
		src = a["url"]
	}
	if caption == "" {
		caption = kind
	}
	if src == "" {
		if kind == "image" && target == nfmTerminal {
			return "🖼 " + caption
		}
		return caption
	}
	if kind == "image" {
		return "![" + caption + "](" + src + ")"
	}
	return "[" + caption + "](" + src + ")"
}
