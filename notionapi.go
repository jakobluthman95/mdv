package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// mdv talks to Notion through one of two independent backends:
//
//   - NOTION_API_TOKEN set: direct HTTPS calls to the public Notion API with that
//     token (an internal integration secret). No other tools needed.
//   - otherwise: the ntn CLI, authenticated with `ntn login`.
//
// Both send the same public API requests; only file uploads differ in how the
// bytes are sent.

const defaultNotionVersion = "2026-03-11"

func notionUsesToken() bool { return os.Getenv("NOTION_API_TOKEN") != "" }

func notionBackendName() string {
	if notionUsesToken() {
		return "Notion API (NOTION_API_TOKEN)"
	}
	return "ntn CLI (ntn login)"
}

// notionRequest calls the public Notion API. path is relative to the API root,
// for example "v1/pages/<id>".
func notionRequest(method, path string, query url.Values, body []byte) ([]byte, error) {
	if notionUsesToken() {
		return notionHTTP(method, path, query, body)
	}
	args := []string{"api", path}
	for k, vs := range query {
		for _, v := range vs {
			args = append(args, k+"=="+v)
		}
	}
	if method != http.MethodGet {
		args = append(args, "-X", method)
	}
	return ntn(body, args...)
}

func notionBaseURL() string {
	if u := os.Getenv("NOTION_API_BASE_URL"); u != "" {
		return strings.TrimRight(u, "/")
	}
	return "https://api.notion.com"
}

func notionVersion() string {
	if v := os.Getenv("NOTION_API_VERSION"); v != "" {
		return v
	}
	return defaultNotionVersion
}

var httpClient = &http.Client{Timeout: 60 * time.Second}

func notionHTTP(method, path string, query url.Values, body []byte) ([]byte, error) {
	u := notionBaseURL() + "/" + strings.TrimLeft(path, "/")
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	return notionDo(func() (*http.Request, error) {
		var r io.Reader
		if body != nil {
			r = bytes.NewReader(body)
		}
		req, err := http.NewRequest(method, u, r)
		if err != nil {
			return nil, err
		}
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		return req, nil
	}, method+" "+path)
}

// notionDo sends a request with auth headers, retrying when rate limited.
func notionDo(build func() (*http.Request, error), label string) ([]byte, error) {
	for attempt := 0; ; attempt++ {
		req, err := build()
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+os.Getenv("NOTION_API_TOKEN"))
		req.Header.Set("Notion-Version", notionVersion())
		res, err := httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("notion %s: %w", label, err)
		}
		out, err := io.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("notion %s: %w", label, err)
		}
		if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			wait, _ := strconv.Atoi(res.Header.Get("Retry-After"))
			time.Sleep(time.Duration(max(wait, 1)) * time.Second)
			continue
		}
		if res.StatusCode >= 300 {
			var e struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}
			if json.Unmarshal(out, &e) == nil && e.Message != "" {
				return nil, fmt.Errorf("notion %s: %s (%d %s)", label, e.Message, res.StatusCode, e.Code)
			}
			return nil, fmt.Errorf("notion %s: HTTP %d", label, res.StatusCode)
		}
		return out, nil
	}
}

// notionUpload uploads a file and returns the upload's ID once Notion reports
// it as uploaded.
func notionUpload(filename, contentType string, data []byte) (string, error) {
	var out []byte
	var err error
	if notionUsesToken() {
		out, err = notionHTTPUpload(filename, contentType, data)
	} else {
		out, err = ntn(data, "files", "create", "--filename", filename, "--content-type", contentType, "--json")
	}
	if err != nil {
		return "", err
	}
	var res struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		return "", err
	}
	if res.ID == "" || res.Status != "uploaded" {
		return "", fmt.Errorf("upload status %q", res.Status)
	}
	return res.ID, nil
}

// notionHTTPUpload mirrors `ntn files create`: create a one-part upload, send
// the bytes as multipart form data, then complete it.
func notionHTTPUpload(filename, contentType string, data []byte) ([]byte, error) {
	create, _ := json.Marshal(map[string]any{
		"filename": filename, "content_type": contentType, "mode": "multi_part", "number_of_parts": 1,
	})
	out, err := notionHTTP(http.MethodPost, "v1/file_uploads", nil, create)
	if err != nil {
		return nil, err
	}
	var upload struct {
		ID        string `json:"id"`
		UploadURL string `json:"upload_url"`
	}
	if err := json.Unmarshal(out, &upload); err != nil {
		return nil, err
	}
	if upload.ID == "" {
		return nil, fmt.Errorf("notion file upload returned no ID: %.200s", out)
	}
	sendURL := upload.UploadURL
	if sendURL == "" {
		sendURL = notionBaseURL() + "/v1/file_uploads/" + upload.ID + "/send"
	}
	if _, err := notionDo(func() (*http.Request, error) {
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename=%q`, filename))
		h.Set("Content-Type", contentType)
		part, err := w.CreatePart(h)
		if err != nil {
			return nil, err
		}
		part.Write(data)
		w.WriteField("part_number", "1")
		w.Close()
		req, err := http.NewRequest(http.MethodPost, sendURL, &buf)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", w.FormDataContentType())
		return req, nil
	}, "send file upload"); err != nil {
		return nil, err
	}
	return notionHTTP(http.MethodPost, "v1/file_uploads/"+upload.ID+"/complete", nil, nil)
}
