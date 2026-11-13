package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

// notionStateFile records every Notion page mdv created. mdv only ever writes to
// pages listed here, so pages it did not create are never touched.
const notionStateFile = ".notion.json"

type notionEntry struct {
	PageID         string `json:"pageId"`
	URL            string `json:"url"`
	ParentID       string `json:"parentId"`
	SHA256         string `json:"sha256"`
	SyncedAt       string `json:"syncedAt"`
	LastEditedTime string `json:"lastEditedTime"`
	LastEditedBy   string `json:"lastEditedBy"`
	Title          string `json:"title"`
	// Images maps an image's content hash to its (attached) Notion file upload.
	Images map[string]string `json:"images,omitempty"`
	// NotionHash fingerprints the page content as Notion returned it after the
	// last push, so edits that leave the content alone (sharing, permissions)
	// don't block the next push.
	NotionHash string `json:"notionHash,omitempty"`
}

// notionState maps a doc's path relative to the docs folder to its Notion page.
type notionState map[string]notionEntry

type syncState int

const (
	notUploaded syncState = iota
	synced
	changed
)

func (s syncState) String() string {
	switch s {
	case synced:
		return "✓ synced"
	case changed:
		return "● changed"
	}
	return "○ local"
}

func loadNotionState(dir string) (notionState, error) {
	b, err := os.ReadFile(filepath.Join(dir, notionStateFile))
	if errors.Is(err, os.ErrNotExist) {
		return notionState{}, nil
	}
	if err != nil {
		return nil, err
	}
	s := notionState{}
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("%s: %w", notionStateFile, err)
	}
	return s, nil
}

func (s notionState) save(dir string) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	p := filepath.Join(dir, notionStateFile)
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func (s notionState) status(dir, rel, path string) syncState {
	e, ok := s[rel]
	if !ok {
		return notUploaded
	}
	if b, err := os.ReadFile(path); err == nil && docHash(dir, path, b) == e.SHA256 {
		return synced
	}
	return changed
}

// --- commands ---

const notionUsage = `Usage:
  mdv notion status                          list docs and their Notion sync state
  mdv notion push <file.md> --parent <page>  create a Notion page under <page> (URL or ID)
  mdv notion push <file.md> --private        create a top-level private Notion page
  mdv notion push <file.md>                  update the page mdv created for <file.md>
  mdv notion diff <file.md>                  show what an update would change (read-only)

  --title <title>  page title (a leading emoji becomes the icon). New pages
                   suggest the doc's H1 (or file name) and ask for a title.
                   Updates keep the title from the last push unless given.
  --yes            skip the prompts (the suggested title is used)

Updates show a diff against the last pushed version before asking.
Local images (![alt](relative/path.png) inside the docs folder) are uploaded to
Notion and placed where they appear in the doc; unchanged images are reused.
push asks for confirmation unless --yes is given. It only updates pages it
created (recorded in <docs>/.notion.json), and refuses if a page was edited in
Notion since the last push.`

func runNotion(dir string, args []string) error {
	if len(args) == 0 {
		fmt.Println(notionUsage)
		return nil
	}
	switch args[0] {
	case "status":
		return notionStatusCmd(dir)
	case "push":
		return notionPushCmd(dir, args[1:])
	case "diff":
		return notionDiffCmd(dir, args[1:])
	case "-h", "--help", "help":
		fmt.Println(notionUsage)
		return nil
	}
	return fmt.Errorf("unknown notion command %q (see mdv notion --help)", args[0])
}

func notionStatusCmd(dir string) error {
	state, err := loadNotionState(dir)
	if err != nil {
		return err
	}
	fmt.Println(dimStyle.Render("Using " + notionBackendName()))
	files := markdownFiles(dir)
	sort.Slice(files, func(i, j int) bool { return files[i].rel < files[j].rel })
	seen := map[string]bool{}
	for _, f := range files {
		seen[f.rel] = true
		line := fmt.Sprintf("%-10s %s", state.status(dir, f.rel, f.path), f.rel)
		if e, ok := state[f.rel]; ok {
			line += "  " + dimStyle.Render(link(e.URL))
		}
		fmt.Println(line)
	}
	for rel, e := range state {
		if !seen[rel] {
			fmt.Printf("%-10s %s  %s\n", "✗ missing", rel, dimStyle.Render(link(e.URL)))
		}
	}
	return nil
}

func notionPushCmd(dir string, args []string) error {
	fs := flag.NewFlagSet("push", flag.ContinueOnError)
	parent := fs.String("parent", "", "parent page URL or ID (new pages only)")
	private := fs.Bool("private", false, "create a top-level private page instead (new pages only)")
	yes := fs.Bool("yes", false, "skip the prompts")
	titleFlag := fs.String("title", "", "page title")
	// Allow flags before and after the file argument.
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return errors.New("push needs a file (see mdv notion --help)")
	}
	arg := fs.Arg(0)
	if err := fs.Parse(fs.Args()[1:]); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return errors.New("push takes exactly one file")
	}

	path, rel, err := resolveDoc(dir, arg)
	if err != nil {
		return err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	icon, title, body := splitTitle(content)
	images, err := localImages(dir, path, content)
	if err != nil {
		return err
	}
	body = withPlaceholders(body)
	state, err := loadNotionState(dir)
	if err != nil {
		return err
	}

	var pageID, parentID string
	var previousImages map[string]string
	uploads := map[string]string{}
	if e, ok := state[rel]; ok {
		if *parent != "" || *private {
			return fmt.Errorf("%s is already in Notion (%s); --parent and --private only apply to new pages", rel, e.URL)
		}
		if state.status(dir, rel, path) == synced {
			seedSnapshot(dir, e.PageID, content)
			fmt.Println("Up to date:", link(e.URL))
			return nil
		}
		page, err := getPage(e.PageID)
		if err != nil {
			return err
		}
		if page.InTrash {
			return fmt.Errorf("the Notion page for %s is in the trash; restore it or remove its entry from %s", rel, notionStateFile)
		}
		if page.LastEditedTime != e.LastEditedTime || page.LastEditedBy.ID != e.LastEditedBy {
			if e.NotionHash == "" {
				return fmt.Errorf("%s was edited in Notion since the last push (%s); not overwriting", e.URL, page.LastEditedTime)
			}
			current, err := pageContentHash(e.PageID)
			if err != nil {
				return err
			}
			if current != e.NotionHash {
				return fmt.Errorf("the content of %s was edited in Notion since the last push (%s); not overwriting", e.URL, page.LastEditedTime)
			}
			fmt.Println(dimStyle.Render("The page changed in Notion since the last push, but not its content (for example sharing or permissions)."))
		}
		switch {
		case *titleFlag != "":
			title = *titleFlag
		case e.Title != "":
			title = e.Title
		}
		diff, err := pageDiff(dir, rel, e, content, body)
		if err != nil {
			return err
		}
		fmt.Print(diff + imageChanges(e, images))
		previousImages = e.Images
		for sha, id := range e.Images {
			uploads[sha] = id
		}
		if err := confirm(fmt.Sprintf("Replace the content of %s (%q) with %s%s?", link(e.URL), title, rel, imageSummary(images, uploads)), *yes); err != nil {
			return err
		}
		if err := uploadImages(images, uploads); err != nil {
			return err
		}
		if err := editPage(e.PageID, body); err != nil {
			return err
		}
		pageID, parentID = e.PageID, e.ParentID
	} else {
		if (*parent == "") == !*private {
			return fmt.Errorf("%s is not in Notion yet; pass either --parent <page URL or ID> or --private", rel)
		}
		where := "your private pages"
		if !*private {
			if parentID, err = parsePageID(*parent); err != nil {
				return err
			}
			p, err := getPage(parentID)
			if err != nil {
				return fmt.Errorf("cannot read parent page: %w", err)
			}
			where = fmt.Sprintf("%q (%s)", p.title(), link(p.URL))
		}
		suggested := *titleFlag
		if suggested == "" {
			suggested = strings.TrimSpace(icon + " " + title)
		}
		if suggested == "" {
			suggested = titleFromFile(rel)
		}
		if title, err = askTitle(suggested, *yes); err != nil {
			return err
		}
		if err := confirm(fmt.Sprintf("Create a new page %q from %s%s under %s?", title, rel, imageSummary(images, uploads), where), *yes); err != nil {
			return err
		}
		// Upload first: a failed upload leaves Notion untouched
		if err := uploadImages(images, uploads); err != nil {
			return err
		}
		if pageID, err = createPage(parentID, title, body); err != nil {
			return err
		}
	}

	if head, tail, ok := strings.Cut(title, " "); ok && isEmoji(head) {
		icon, title = head, strings.TrimSpace(tail)
	}
	if err := setTitleIcon(pageID, title, icon); err != nil {
		fmt.Fprintln(os.Stderr, "mdv: content pushed, but setting the title/icon failed:", err)
	}
	// The page is written either way: record it, but leave it "changed" when
	// images failed so the next push retries
	imageErr := attachImages(pageID, images, uploads)
	hash, attached := docHash(dir, path, content), map[string]string{}
	for _, image := range images {
		attached[image.sha] = uploads[image.sha]
	}
	if imageErr != nil {
		hash, attached = "", previousImages
	}

	// Read the page back so the next push can tell whether anyone else edited it.
	notionHash, err := pageContentHash(pageID)
	if err != nil {
		fmt.Fprintln(os.Stderr, "mdv: pushed, but could not fingerprint the page content:", err)
	}
	page, err := getPage(pageID)
	if err != nil {
		return fmt.Errorf("page written but could not be read back: %w", err)
	}
	state[rel] = notionEntry{
		PageID:         page.ID,
		URL:            page.URL,
		ParentID:       parentID,
		SHA256:         hash,
		SyncedAt:       time.Now().UTC().Format(time.RFC3339),
		LastEditedTime: page.LastEditedTime,
		LastEditedBy:   page.LastEditedBy.ID,
		Title:          strings.TrimSpace(icon + " " + title),
		Images:         attached,
		NotionHash:     notionHash,
	}
	if err := state.save(dir); err != nil {
		return err
	}
	if err := saveSnapshot(dir, page.ID, content); err != nil {
		fmt.Fprintln(os.Stderr, "mdv: pushed, but saving the snapshot for the next diff failed:", err)
	}
	if imageErr != nil {
		return fmt.Errorf("page pushed (%s), but images failed: %w; push again to retry", page.URL, imageErr)
	}
	fmt.Println("Pushed:", link(page.URL))
	return nil
}

// imageSummary describes the images a push includes, for the confirmation.
func imageSummary(images []localImage, uploads map[string]string) string {
	if len(images) == 0 {
		return ""
	}
	pending := map[string]bool{}
	for _, image := range images {
		if _, ok := uploads[image.sha]; !ok {
			pending[image.sha] = true
		}
	}
	return fmt.Sprintf(" with %d images (%d to upload)", len(images), len(pending))
}

func notionDiffCmd(dir string, args []string) error {
	if len(args) != 1 {
		return errors.New("diff takes exactly one file")
	}
	path, rel, err := resolveDoc(dir, args[0])
	if err != nil {
		return err
	}
	state, err := loadNotionState(dir)
	if err != nil {
		return err
	}
	e, ok := state[rel]
	if !ok {
		return fmt.Errorf("%s is not in Notion yet", rel)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if state.status(dir, rel, path) == synced {
		seedSnapshot(dir, e.PageID, content)
		fmt.Println("No changes since the last push:", link(e.URL))
		return nil
	}
	images, err := localImages(dir, path, content)
	if err != nil {
		return err
	}
	_, _, body := splitTitle(content)
	diff, err := pageDiff(dir, rel, e, content, withPlaceholders(body))
	if err != nil {
		return err
	}
	fmt.Print(diff + imageChanges(e, images))
	return nil
}

// resolveDoc resolves a doc path. Relative paths are relative to the docs
// folder, not the working directory, and files outside it are refused.
func resolveDoc(dir, arg string) (path, rel string, err error) {
	if !filepath.IsAbs(arg) && !strings.HasPrefix(arg, "~") {
		arg = filepath.Join(dir, arg)
	}
	if path, err = resolveFile(arg); err != nil {
		return "", "", err
	}
	rel, err = filepath.Rel(dir, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", "", fmt.Errorf("%s is outside the docs folder %s", arg, tildify(dir))
	}
	return path, rel, nil
}

// Snapshots of the last pushed content live in <docs>/.notion/<page ID>.md.
// mdv refuses to push over edits made in Notion, so a snapshot always matches
// what is on the page.
func snapshotPath(dir, pageID string) string {
	return filepath.Join(dir, ".notion", pageID+".md")
}

func saveSnapshot(dir, pageID string, content []byte) error {
	p := snapshotPath(dir, pageID)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, content, 0o644)
}

// seedSnapshot saves a missing snapshot for a synced doc: its content is
// exactly what was last pushed.
func seedSnapshot(dir, pageID string, content []byte) {
	if _, err := os.Stat(snapshotPath(dir, pageID)); errors.Is(err, os.ErrNotExist) {
		_ = saveSnapshot(dir, pageID, content)
	}
}

// pageDiff shows what pushing content would change on the page. Without a
// snapshot (pages pushed before snapshots existed) it compares the body with
// Notion's own Markdown export, which also shows formatting differences.
func pageDiff(dir, rel string, e notionEntry, content, body []byte) (string, error) {
	old, err := os.ReadFile(snapshotPath(dir, e.PageID))
	note := ""
	if errors.Is(err, os.ErrNotExist) {
		if old, err = getMarkdown(e.PageID); err != nil {
			return "", err
		}
		content = body
		note = dimStyle.Render("No snapshot from an earlier push: comparing with Notion's Markdown export, so formatting differences show too.") + "\n"
	} else if err != nil {
		return "", err
	}
	diff, err := gitDiff(rel, old, content)
	if err != nil || diff == "" {
		return "", err
	}
	return note + diff, nil
}

// gitDiff renders a git-style diff of old (as on Notion) against new (local).
func gitDiff(rel string, old, new []byte) (string, error) {
	tmp, err := os.MkdirTemp("", "mdv-diff-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	a, b := filepath.Join("notion", rel), filepath.Join("local", rel)
	for name, data := range map[string][]byte{a: old, b: new} {
		p := filepath.Join(tmp, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			return "", err
		}
	}
	color := "--color=never"
	if fi, err := os.Stdout.Stat(); err == nil && fi.Mode()&os.ModeCharDevice != 0 {
		color = "--color=always"
	}
	cmd := exec.Command("git", "diff", "--no-index", color, a, b)
	cmd.Dir = tmp
	out, err := cmd.Output()
	// Exit code 1 means the files differ.
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		err = nil
	}
	return string(out), err
}

func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

var stdin = bufio.NewReader(os.Stdin)

func isTerminal() bool {
	fi, err := os.Stdin.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// askTitle offers the suggested title and lets the user type another one.
// Every new page gets a title.
func askTitle(suggested string, yes bool) (string, error) {
	if !yes && isTerminal() {
		fmt.Printf("Title [%s]: ", suggested)
		answer, _ := stdin.ReadString('\n')
		if a := strings.TrimSpace(answer); a != "" {
			return a, nil
		}
	}
	if suggested == "" {
		return "", errors.New("new pages need a title; pass --title")
	}
	return suggested, nil
}

// titleFromFile turns "TAP-123-some-topic.md" into "TAP-123 some topic".
func titleFromFile(rel string) string {
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

func confirm(question string, yes bool) error {
	if yes {
		return nil
	}
	if !isTerminal() {
		return errors.New("no terminal to confirm on; pass --yes to skip the prompt")
	}
	fmt.Print(question + " [y/N] ")
	answer, _ := stdin.ReadString('\n')
	if a := strings.ToLower(strings.TrimSpace(answer)); a != "y" && a != "yes" {
		return errors.New("cancelled, nothing was written")
	}
	return nil
}

// link makes url clickable (OSC 8) when stdout is a terminal.
func link(url string) string {
	if fi, err := os.Stdout.Stat(); err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return url
	}
	return "\x1b]8;;" + url + "\x1b\\" + url + "\x1b]8;;\x1b\\"
}

// parsePageID accepts a page ID (with or without dashes) or a Notion page URL.
func parsePageID(s string) (string, error) {
	s, _, _ = strings.Cut(s, "?")
	s, _, _ = strings.Cut(s, "#")
	s = strings.ReplaceAll(s, "-", "")
	if len(s) >= 32 {
		id := strings.ToLower(s[len(s)-32:])
		if _, err := hex.DecodeString(id); err == nil {
			return id[:8] + "-" + id[8:12] + "-" + id[12:16] + "-" + id[16:20] + "-" + id[20:], nil
		}
	}
	return "", fmt.Errorf("cannot find a Notion page ID in %q", s)
}

// --- Notion API (see notionapi.go for the backends) ---

type notionPage struct {
	ID             string `json:"id"`
	URL            string `json:"url"`
	InTrash        bool   `json:"in_trash"`
	LastEditedTime string `json:"last_edited_time"`
	LastEditedBy   struct {
		ID string `json:"id"`
	} `json:"last_edited_by"`
	Properties map[string]struct {
		Type  string `json:"type"`
		Title []struct {
			PlainText string `json:"plain_text"`
		} `json:"title"`
	} `json:"properties"`
}

func (p notionPage) title() string {
	for _, prop := range p.Properties {
		if prop.Type == "title" {
			var b strings.Builder
			for _, t := range prop.Title {
				b.WriteString(t.PlainText)
			}
			if b.Len() > 0 {
				return b.String()
			}
		}
	}
	return "Untitled"
}

func ntn(stdin []byte, args ...string) ([]byte, error) {
	cmd := exec.Command("ntn", args...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("ntn %s: %s", args[0], msg)
		}
		return nil, fmt.Errorf("ntn %s: %w", args[0], err)
	}
	return out, nil
}

func getMarkdown(id string) ([]byte, error) {
	out, err := notionRequest("GET", "v1/pages/"+id+"/markdown", nil, nil)
	if err != nil {
		return nil, err
	}
	var res struct {
		Markdown string `json:"markdown"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		return nil, err
	}
	md := res.Markdown
	if md != "" && !strings.HasSuffix(md, "\n") {
		md += "\n"
	}
	return []byte(md), nil
}

// pageContentHash fingerprints a page's content. Image links are left out:
// Notion returns a new signed URL for every read.
func pageContentHash(id string) (string, error) {
	md, err := getMarkdown(id)
	if err != nil {
		return "", err
	}
	return hashBytes(imageURLs.ReplaceAll(md, []byte("![$1]()"))), nil
}

var imageURLs = regexp.MustCompile(`!\[([^\]]*)\]\([^)]*\)`)

func getPage(id string) (notionPage, error) {
	var p notionPage
	out, err := notionRequest("GET", "v1/pages/"+id, nil, nil)
	if err != nil {
		return p, err
	}
	return p, json.Unmarshal(out, &p)
}

// splitTitle takes the leading "# H1" off a doc so it can be sent as the page
// title instead of a heading block. A leading emoji in the H1 becomes the icon.
func splitTitle(markdown []byte) (icon, title string, body []byte) {
	text := strings.TrimLeft(string(markdown), " \t\r\n")
	first, rest, _ := strings.Cut(text, "\n")
	if !strings.HasPrefix(first, "# ") {
		return "", "", markdown
	}
	title = strings.TrimSpace(strings.TrimPrefix(first, "# "))
	if head, tail, ok := strings.Cut(title, " "); ok && isEmoji(head) {
		icon, title = head, strings.TrimSpace(tail)
	}
	return icon, title, []byte(strings.TrimLeft(rest, "\r\n"))
}

func isEmoji(s string) bool {
	for _, r := range s {
		if r < 0x80 || unicode.IsLetter(r) || unicode.IsDigit(r) {
			return false
		}
	}
	return s != ""
}

// createPage creates a page under parentID, or a top-level private page when
// parentID is empty.
func createPage(parentID, title string, body []byte) (string, error) {
	req := map[string]any{"markdown": string(body)}
	if parentID != "" {
		req["parent"] = map[string]string{"page_id": parentID}
	}
	if title != "" {
		req["properties"] = map[string]any{"title": map[string]any{"title": []any{map[string]any{"type": "text", "text": map[string]string{"content": title}}}}}
	}
	payload, _ := json.Marshal(req)
	out, err := notionRequest("POST", "v1/pages", nil, payload)
	if err != nil {
		return "", err
	}
	var res struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(out, &res); err != nil {
		return "", err
	}
	if res.ID == "" {
		return "", fmt.Errorf("creating the page returned no page ID: %.200s", out)
	}
	return res.ID, nil
}

// editPage replaces a page's content. It never deletes child pages or databases.
func editPage(id string, body []byte) error {
	payload, _ := json.Marshal(map[string]any{"type": "replace_content", "replace_content": map[string]string{"new_str": string(body)}})
	_, err := notionRequest("PATCH", "v1/pages/"+id+"/markdown", nil, payload)
	return err
}

// setTitleIcon sets the title and icon of a page mdv created.
func setTitleIcon(id, title, icon string) error {
	props := map[string]any{}
	if title != "" {
		props["properties"] = map[string]any{"title": map[string]any{"title": []any{map[string]any{"text": map[string]string{"content": title}}}}}
	}
	if icon != "" {
		props["icon"] = map[string]string{"type": "emoji", "emoji": icon}
	}
	if len(props) == 0 {
		return nil
	}
	body, _ := json.Marshal(props)
	_, err := notionRequest("PATCH", "v1/pages/"+id, nil, body)
	return err
}
