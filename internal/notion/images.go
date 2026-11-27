package notion

import (
	"encoding/json"
	"fmt"
	"mime"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/jakobluthman95/mdv/internal/docs"
)

// Notion's Markdown cannot reference uploaded files, so local images are pushed
// as empty image placeholders (`![alt]()`) and each placeholder block is then
// pointed at a file upload (`ntn files create`). Once attached, an upload stays
// valid and can be attached again, so uploads are cached by content hash and
// reused on later pushes.

// imageChanges lists images that are new or changed since the last push.
func imageChanges(e notionEntry, images []docs.Image) string {
	var b strings.Builder
	for _, image := range images {
		if _, ok := e.Images[image.SHA]; !ok {
			fmt.Fprintf(&b, "image new or changed: %s\n", image.Ref)
		}
	}
	return b.String()
}

// uploadImages uploads the images that have no cached upload yet.
func uploadImages(images []docs.Image, uploads map[string]string) error {
	for _, image := range images {
		if _, ok := uploads[image.SHA]; ok {
			continue
		}
		id, err := uploadImage(image)
		if err != nil {
			return err
		}
		uploads[image.SHA] = id
	}
	return nil
}

func uploadImage(image docs.Image) (string, error) {
	contentType := mime.TypeByExtension(strings.ToLower(filepath.Ext(image.Path)))
	if !strings.HasPrefix(contentType, "image/") {
		return "", fmt.Errorf("image %s: unsupported file type", image.Ref)
	}
	b, err := os.ReadFile(image.Path)
	if err != nil {
		return "", err
	}
	id, err := notionUpload(filepath.Base(image.Path), contentType, b)
	if err != nil {
		return "", fmt.Errorf("uploading %s: %w", image.Ref, err)
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
func attachImages(pageID string, images []docs.Image, uploads map[string]string) error {
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
		err := setImageBlock(blocks[i], uploads[image.SHA], image.Alt)
		if err != nil {
			id, uploadErr := uploadImage(image)
			if uploadErr != nil {
				return fmt.Errorf("attaching %s: %w", image.Ref, err)
			}
			uploads[image.SHA] = id
			err = setImageBlock(blocks[i], id, image.Alt)
		}
		if err != nil {
			return fmt.Errorf("attaching %s: %w", image.Ref, err)
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
