package docs

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jakobluthman95/mdv/internal/term"
)

var imageRef = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)

type Image struct {
	Alt  string
	Ref  string // as written in the doc
	Path string
	SHA  string
}

func isRemote(ref string) bool {
	return strings.Contains(ref, "://") || strings.HasPrefix(ref, "data:")
}

// LocalImages returns the images a doc references by path, in document order.
// Images must exist inside the docs folder.
func LocalImages(dir, docPath string, content []byte) ([]Image, error) {
	var images []Image
	for _, m := range imageRef.FindAllSubmatch(content, -1) {
		ref := string(m[2])
		if isRemote(ref) {
			continue
		}
		unescaped, err := url.PathUnescape(ref)
		if err != nil {
			unescaped = ref
		}
		p := filepath.Clean(filepath.Join(filepath.Dir(docPath), unescaped))
		if rel, err := filepath.Rel(dir, p); err != nil || strings.HasPrefix(rel, "..") {
			return nil, fmt.Errorf("image %s is outside the docs folder %s", ref, term.Tildify(dir))
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("image %s: %w", ref, err)
		}
		images = append(images, Image{Alt: string(m[1]), Ref: ref, Path: p, SHA: HashBytes(b)})
	}
	return images, nil
}

// WithPlaceholders replaces local image references with empty ones.
func WithPlaceholders(body []byte) []byte {
	return imageRef.ReplaceAllFunc(body, func(m []byte) []byte {
		sub := imageRef.FindSubmatch(m)
		if isRemote(string(sub[2])) {
			return m
		}
		return []byte("![" + string(sub[1]) + "]()")
	})
}

// WithRawURLs points local image references at the URLs in urls (keyed by the
// reference as written in the doc). Other references are left alone.
func WithRawURLs(body []byte, urls map[string]string) []byte {
	return imageRef.ReplaceAllFunc(body, func(m []byte) []byte {
		sub := imageRef.FindSubmatch(m)
		u, ok := urls[string(sub[2])]
		if !ok {
			return m
		}
		return []byte("![" + string(sub[1]) + "](" + u + ")")
	})
}

// Hash covers a doc and the local images it references, so a changed image
// counts as a change. Docs without local images hash to their content hash.
func Hash(dir, path string, content []byte) string {
	content = StripFrontmatter(content)
	images, err := LocalImages(dir, path, content)
	if err != nil || len(images) == 0 {
		return HashBytes(content)
	}
	h := sha256.New()
	h.Write(content)
	for _, image := range images {
		fmt.Fprintf(h, "\n%s %s", image.Ref, image.SHA)
	}
	return hex.EncodeToString(h.Sum(nil))
}
