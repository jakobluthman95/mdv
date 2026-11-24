package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

var tableInternals = map[string]bool{"colgroup": true, "col": true, "tr": true, "th": true, "td": true}

func referenceTags(t *testing.T) []string {
	b, err := os.ReadFile("skills/mdv-docs/notion-markdown.md")
	if err != nil {
		t.Fatal(err)
	}
	doc := string(b)
	start, end := strings.Index(doc, "## 1."), strings.Index(doc, "## 3.")
	if start < 0 || end < start {
		t.Fatal("reference doc has no groups 1-2")
	}
	seen := map[string]bool{}
	var tags []string
	tag := regexp.MustCompile("`[^`]*?<([a-z][a-z0-9-]*)")
	for _, line := range strings.Split(doc[start:end], "\n") {
		cols := strings.Split(line, " | ")
		if !strings.HasPrefix(line, "|") || len(cols) < 2 {
			continue
		}
		for _, m := range regexp.MustCompile("`[^`]+`").FindAllString(cols[1], -1) {
			for _, sub := range tag.FindAllStringSubmatch(m, -1) {
				if name := sub[1]; !seen[name] && !tableInternals[name] {
					seen[name] = true
					tags = append(tags, name)
				}
			}
		}
	}
	if len(tags) < 20 {
		t.Fatalf("found only %d tags in the reference: %v", len(tags), tags)
	}
	return tags
}

func TestEveryReferenceTagHasFallback(t *testing.T) {
	handled := map[string]bool{}
	for _, n := range append(append([]string{}, nfmBlockTags...), nfmInlineTags...) {
		handled[n] = true
	}
	var fixtures strings.Builder
	for _, f := range []string{"testdata/nfm-notion-readback.md", "testdata/nfm-extra.md"} {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		fixtures.Write(b)
		fixtures.WriteString("\n")
	}
	for _, tag := range referenceTags(t) {
		if !handled[tag] {
			t.Errorf("<%s> is in the reference but has no fallback", tag)
		}
		if !strings.Contains(fixtures.String(), "<"+tag) {
			t.Errorf("<%s> is not covered by the test fixtures", tag)
		}
	}
	for name, out := range map[string]string{
		"terminal": string(nfmForTerminal([]byte(fixtures.String()))),
		"gist":     string(nfmForGist([]byte(fixtures.String()))),
	} {
		outside := regexp.MustCompile("(?s)```.*?```|`[^`]*`").ReplaceAllString(out, "")
		for _, tag := range referenceTags(t) {
			if strings.Contains(outside, "<"+tag+">") || strings.Contains(outside, "<"+tag+" ") || strings.Contains(outside, "</"+tag+">") {
				t.Errorf("%s output still contains <%s>:\n%s", name, tag, out)
			}
		}
	}
}

func TestPlainMarkdownUnchanged(t *testing.T) {
	plain := "# 🐛 Title\n\n- **Repo:** `web-monorepo`\n\n## Findings\n\n> 💡 A tip with `<toggle>` in code.\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n- [ ] todo\n\n![shot](doc/shot.png)\n\n```html\n<callout icon=\"x\">\nstays\n</callout>\n```\n\nText with a < sign and a [link](https://example.com).\n"
	for name, f := range map[string]func([]byte) []byte{"terminal": nfmForTerminal, "gist": nfmForGist} {
		if got := string(f([]byte(plain))); got != plain {
			t.Errorf("%s changed plain Markdown:\n%s", name, got)
		}
	}
}

func visibleTerminal(t *testing.T, md string) string {
	t.Helper()
	out, err := renderForTerminal(md, "dark", 80)
	if err != nil {
		t.Fatal(err)
	}
	out = regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(out, "")
	var lines []string
	for _, l := range strings.Split(out, "\n") {
		if l = strings.TrimRight(l, " "); strings.Trim(l, " ┊") != "" {
			lines = append(lines, strings.TrimPrefix(l, "  "))
		}
	}
	return strings.Join(lines, "\n")
}

func TestNFMTerminal(t *testing.T) {
	cases := []struct{ in, want string }{
		{"<toggle>\nTitle\n\nBody\n</toggle>\n\nAfter", "▸ Title\n┊ Body\nAfter"},
		{"<toggle heading=\"h2\">\nTitle\n\nBody\n</toggle>", "▌ ▸ Title\n┊ Body"},
		{"<callout icon=\"💡\" color=\"yellow_bg\">\nTip\n\nMore\n</callout>", "│ 💡 Tip\n│\n│ More"},
		{"<toggle>\nOuter\n\n<toggle>\nInner\n\nDeep\n</toggle>\n</toggle>", "▸ Outer\n┊ ▸ Inner\n┊ ┊ Deep"},
		{"<columns>\n<column>\nLeft\n</column>\n<column>\nRight\n</column>\n</columns>", "◫ Column 1\n┊ Left\n◫ Column 2\n┊ Right"},
		{"<tabs>\n<tab icon=\"📘\">\nOne\n\nBody one\n</tab>\n</tabs>", "📘 One\n┊ Body one"},
		{"<todo checked=\"true\" color=\"green\">\nTask\n</todo>", "[✓] Task"},
		{"A <span color=\"red\"><u>red</u></span> word", "A red word"},
		{"<image>\ncaption\n</image>", "🖼 caption"},
		{"<table header-row=\"true\">\n<tr>\n<td>\n<span color=\"red\">A</span>\n</td>\n<td>\nB\n</td>\n</tr>\n</table>", ""},
	}
	for _, c := range cases {
		got := visibleTerminal(t, c.in)
		if c.want != "" && got != c.want {
			t.Errorf("%q:\ngot\n%s\nwant\n%s", c.in, got, c.want)
		}
		if strings.ContainsFunc(got, func(r rune) bool { return r >= 0xE000 && r <= 0xF8FF }) {
			t.Errorf("%q: marker characters left in output:\n%q", c.in, got)
		}
	}
}

func TestNFMTerminalColors(t *testing.T) {
	out, err := renderForTerminal("A <span color=\"red\">red</span>, <u>under</u> and <span color=\"blue_bg\">blue</span>.", "dark", 80)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"\x1b[38;5;203mred\x1b[39;49;24m", "\x1b[4munder\x1b[39;49;24m", "\x1b[48;5;24mblue\x1b[39;49;24m"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %q", want, out)
		}
	}
}

func TestNFMGist(t *testing.T) {
	cases := []struct{ in, want string }{
		{"<toggle>\nTitle\n\nBody\n</toggle>", "<details><summary>Title</summary>\n\nBody\n\n</details>"},
		{"<toggle heading=\"h2\">\nTitle\n\nBody\n</toggle>", "<details><summary><strong>Title</strong></summary>\n\nBody\n\n</details>"},
		{"<callout icon=\"💡\" color=\"yellow_bg\">\nTip\n\nMore\n</callout>", "> 💡 Tip\n>\n> More"},
		{"<todo checked=\"false\">\nTask\n</todo>", "- [ ] Task"},
		{"A <span color=\"red\"><u>red</u></span> word", "A <ins>red</ins> word"},
		{"`<span>kept</span>` and <span>gone</span>", "`<span>kept</span>` and gone"},
		{"<mention url=\"https://x.io/p\">P</mention>", "[P](https://x.io/p)"},
		{"<pdf source=\"https://x.io/d.pdf\" />", "[pdf](https://x.io/d.pdf)"},
		{"<image>\ncaption\n</image>", "caption"},
		{"<page url=\"https://x.io/p\">Sub</page>", "<page url=\"https://x.io/p\">Sub</page>"},
	}
	for _, c := range cases {
		if got := string(nfmForGist([]byte(c.in))); got != c.want {
			t.Errorf("gist %q:\ngot  %q\nwant %q", c.in, got, c.want)
		}
	}
}

func TestStyledTable(t *testing.T) {
	in := "<table header-row=\"true\">\n<tr>\n<th>\nA\n</th>\n<th>\nB\n</th>\n</tr>\n<tr>\n<td color=\"red\">\nx | y\n</td>\n<td>\none<br />two\n</td>\n</tr>\n</table>"
	want := "| A | B |\n| --- | --- |\n| x \\| y | one two |"
	if got := string(nfmForGist([]byte(in))); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestMarkdownVersionForHash(t *testing.T) {
	if v := (notionEntry{}).markdownVersion(); v != "v1" {
		t.Errorf("entries from before v2 should compare in v1, got %q", v)
	}
	if v := (notionEntry{MarkdownVersion: "v2"}).markdownVersion(); v != "v2" {
		t.Errorf("got %q", v)
	}
	a := sourceURLs.ReplaceAllString(`<image source="https://s3/sig=1">c</image>`, `source=""`)
	b := sourceURLs.ReplaceAllString(`<image source="https://s3/sig=2">c</image>`, `source=""`)
	if a != b {
		t.Errorf("signed image URLs should not change the fingerprint: %q vs %q", a, b)
	}
}
