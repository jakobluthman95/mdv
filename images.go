package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Notion's Markdown cannot reference uploaded files, so local images are pushed
// as empty image placeholders (`![alt]()`) and each placeholder block is then
// pointed at a file upload (`ntn files create`). Once attached, an upload stays
// valid and can be attached again, so uploads are cached by content hash and
// reused on later pushes.

var imageRef = regexp.MustCompile(`!\[([^\]]*)\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)

type localImage struct {
	alt  string
	ref  string // as written in the doc
	path string
	sha  string
}

func isRemote(ref string) bool {
	return strings.Contains(ref, "://") || strings.HasPrefix(ref, "data:")
}

// localImages returns the images a doc references by path, in document order.
// Images must exist inside the docs folder.
func localImages(dir, docPath string, content []byte) ([]localImage, error) {
	var images []localImage
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
			return nil, fmt.Errorf("image %s is outside the docs folder %s", ref, tildify(dir))
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("image %s: %w", ref, err)
		}
		images = append(images, localImage{alt: string(m[1]), ref: ref, path: p, sha: hashBytes(b)})
	}
	return images, nil
}

// withPlaceholders replaces local image references with empty ones.
func withPlaceholders(body []byte) []byte {
	return imageRef.ReplaceAllFunc(body, func(m []byte) []byte {
		sub := imageRef.FindSubmatch(m)
		if isRemote(string(sub[2])) {
			return m
		}
		return []byte("![" + string(sub[1]) + "]()")
	})
}

// withRawURLs points local image references at the URLs in urls (keyed by the
// reference as written in the doc). Other references are left alone.
func withRawURLs(body []byte, urls map[string]string) []byte {
	return imageRef.ReplaceAllFunc(body, func(m []byte) []byte {
		sub := imageRef.FindSubmatch(m)
		u, ok := urls[string(sub[2])]
		if !ok {
			return m
		}
		return []byte("![" + string(sub[1]) + "](" + u + ")")
	})
}

// docHash covers a doc and the local images it references, so a changed image
// counts as a change. Docs without local images hash to their content hash.
func docHash(dir, path string, content []byte) string {
	images, err := localImages(dir, path, content)
	if err != nil || len(images) == 0 {
		return hashBytes(content)
	}
	h := sha256.New()
	h.Write(content)
	for _, image := range images {
		fmt.Fprintf(h, "\n%s %s", image.ref, image.sha)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// imageChanges lists images that are new or changed since the last push.
func imageChanges(e notionEntry, images []localImage) string {
	var b strings.Builder
	for _, image := range images {
		if _, ok := e.Images[image.sha]; !ok {
			fmt.Fprintf(&b, "image new or changed: %s\n", image.ref)
		}
	}
	return b.String()
}

// uploadImages uploads the images that have no cached upload yet.
func uploadImages(images []localImage, uploads map[string]string) error {
	for _, image := range images {
		if _, ok := uploads[image.sha]; ok {
			continue
		}
		id, err := uploadImage(image)
		if err != nil {
			return err
		}
		uploads[image.sha] = id
	}
	return nil
}

func uploadImage(image localImage) (string, error) {
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(image.path)))
	if !strings.HasPrefix(contentType, "image/") {
		return "", fmt.Errorf("image %s: unsupported file type", image.ref)
	}
	b, err := os.ReadFile(image.path)
	if err != nil {
		return "", err
	}
	id, err := notionUpload(filepath.Base(image.path), contentType, b)
	if err != nil {
		return "", fmt.Errorf("uploading %s: %w", image.ref, err)
	}
	return id, nil
}

type notionBlock struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Image struct {
		External struct {
			URL string `json:"url"`
		} `json:"external"`
		File struct {
			URL string `json:"url"`
		} `json:"file"`
	} `json:"image"`
}

// placeholderBlocks returns the page's empty image blocks, in order.
func placeholderBlocks(pageID string) ([]string, error) {
	var ids []string
	cursor := ""
	for {
		query := url.Values{"page_size": {"100"}}
		if cursor != "" {
			query.Set("start_cursor", cursor)
		}
		out, err := notionRequest("GET", "v1/blocks/"+pageID+"/children", query, nil)
		if err != nil {
			return nil, err
		}
		var res struct {
			Results    []notionBlock `json:"results"`
			HasMore    bool          `json:"has_more"`
			NextCursor string        `json:"next_cursor"`
		}
		if err := json.Unmarshal(out, &res); err != nil {
			return nil, err
		}
		for _, block := range res.Results {
			if block.Type == "image" && block.Image.External.URL == "" && block.Image.File.URL == "" {
				ids = append(ids, block.ID)
			}
		}
		if !res.HasMore || res.NextCursor == "" {
			return ids, nil
		}
		cursor = res.NextCursor
	}
}

// attachImages points the page's image placeholders at the uploads, in
// document order. A cached upload that cannot be attached is uploaded again.
func attachImages(pageID string, images []localImage, uploads map[string]string) error {
	if len(images) == 0 {
		return nil
	}
	blocks, err := placeholderBlocks(pageID)
	if err != nil {
		return err
	}
	if len(blocks) != len(images) {
		return fmt.Errorf("found %d image placeholders on the page for %d images", len(blocks), len(images))
	}
	for i, image := range images {
		err := setImageBlock(blocks[i], uploads[image.sha], image.alt)
		if err != nil {
			id, uploadErr := uploadImage(image)
			if uploadErr != nil {
				return fmt.Errorf("attaching %s: %w", image.ref, err)
			}
			uploads[image.sha] = id
			err = setImageBlock(blocks[i], id, image.alt)
		}
		if err != nil {
			return fmt.Errorf("attaching %s: %w", image.ref, err)
		}
	}
	return nil
}

func setImageBlock(blockID, uploadID, caption string) error {
	image := map[string]any{"file_upload": map[string]string{"id": uploadID}}
	if caption != "" {
		image["caption"] = []any{map[string]any{"type": "text", "text": map[string]string{"content": caption}}}
	}
	body, _ := json.Marshal(map[string]any{"image": image})
	_, err := notionRequest("PATCH", "v1/blocks/"+blockID, nil, body)
	return err
}
