package notion

import "testing"

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
