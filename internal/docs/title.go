package docs

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/jakobluthman95/mdv/internal/term"
)

var TicketInName = regexp.MustCompile(`^([A-Za-z][A-Za-z0-9]+-\d+)`)

// ResolveDoc resolves a doc path. Relative paths are relative to the docs
// folder, not the working directory, and files outside it are refused.
func ResolveDoc(dir, arg string) (path, rel string, err error) {
	if !filepath.IsAbs(arg) && !strings.HasPrefix(arg, "~") {
		arg = filepath.Join(dir, arg)
	}
	if path, err = ResolveFile(arg); err != nil {
		return "", "", err
	}
	rel, err = filepath.Rel(dir, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", "", fmt.Errorf("%s is outside the docs folder %s", arg, term.Tildify(dir))
	}
	return path, rel, nil
}

func HashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// TitleFromFile turns "TAP-123-some-topic.md" into "TAP-123 some topic".
func TitleFromFile(rel string) string {
	name := strings.TrimSuffix(filepath.Base(rel), filepath.Ext(rel))
	words := strings.FieldsFunc(name, func(r rune) bool { return r == '-' || r == '_' })
	if len(words) >= 2 && strings.ToUpper(words[0]) == words[0] && isNumber(words[1]) {
		words = append([]string{words[0] + "-" + words[1]}, words[2:]...)
	}
	return strings.Join(words, " ")
}

func isNumber(s string) bool {
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return s != ""
}

// SplitTitle takes the leading "# H1" off a doc so it can be sent as the page
// title instead of a heading block. A leading emoji in the H1 becomes the icon.
func SplitTitle(markdown []byte) (icon, title string, body []byte) {
	text := strings.TrimLeft(string(markdown), " \t\r\n")
	first, rest, _ := strings.Cut(text, "\n")
	if !strings.HasPrefix(first, "# ") {
		return "", "", markdown
	}
	title = strings.TrimSpace(strings.TrimPrefix(first, "# "))
	if head, tail, ok := strings.Cut(title, " "); ok && IsEmoji(head) {
		icon, title = head, strings.TrimSpace(tail)
	}
	return icon, title, []byte(strings.TrimLeft(rest, "\r\n"))
}

func IsEmoji(s string) bool {
	for _, r := range s {
		if r < 0x80 || unicode.IsLetter(r) || unicode.IsDigit(r) {
			return false
		}
	}
	return s != ""
}

// StripEmoji drops a leading emoji word, e.g. "⚙️ Systems" -> "Systems".
func StripEmoji(s string) string {
	if head, tail, ok := strings.Cut(s, " "); ok && IsEmoji(head) {
		return strings.TrimSpace(tail)
	}
	return s
}

// Title is the doc's H1, or a title made from its file name.
func Title(rel string, content []byte) string {
	icon, title, _ := SplitTitle(content)
	if t := strings.TrimSpace(icon + " " + title); t != "" {
		return t
	}
	return TitleFromFile(rel)
}
