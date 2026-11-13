package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Sharing publishes a doc as a secret (unlisted) GitHub gist through the gh
// CLI: anyone with the link can read it, nobody else can find it. Local images
// are committed into the gist's git repo and the uploaded copy of the doc links
// to their raw URLs. The local doc is never changed.

// shareStateFile records every gist mdv created. mdv only ever updates or
// deletes gists listed here.
const shareStateFile = ".share.json"

type shareEntry struct {
	GistID   string `json:"gistId"`
	URL      string `json:"url"`
	Owner    string `json:"owner"`
	File     string `json:"file"` // the doc's file name inside the gist
	Title    string `json:"title"`
	SHA256   string `json:"sha256"`
	SharedAt string `json:"sharedAt"`
	// Images maps an image's content hash to its file name inside the gist.
	Images map[string]string `json:"images,omitempty"`
}

type shareState map[string]shareEntry

func loadShareState(dir string) (shareState, error) {
	b, err := os.ReadFile(filepath.Join(dir, shareStateFile))
	if errors.Is(err, os.ErrNotExist) {
		return shareState{}, nil
	}
	if err != nil {
		return nil, err
	}
	s := shareState{}
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, fmt.Errorf("%s: %w", shareStateFile, err)
	}
	return s, nil
}

func (s shareState) save(dir string) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	p := filepath.Join(dir, shareStateFile)
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

const shareUsage = `Usage:
  mdv share <file.md>      share a doc as a secret GitHub gist (only people with
                           the link can open it); updates the same gist if the
                           doc was shared before
  mdv share status         list shared docs and whether they changed since
  mdv share rm <file.md>   delete the gist and forget it

  --yes  skip the confirmation prompt

Gists are created with the gh CLI on the account it is logged in to. Local
images (![alt](relative/path.png) inside the docs folder) are uploaded into the
gist and linked by URL in the shared copy; your doc is never changed. Shares are
recorded in <docs>/.share.json, and mdv only touches gists listed there.`

func runShare(dir string, args []string) error {
	if len(args) == 0 {
		fmt.Println(shareUsage)
		return nil
	}
	switch args[0] {
	case "-h", "--help", "help":
		fmt.Println(shareUsage)
		return nil
	case "status":
		return shareStatusCmd(dir)
	case "rm":
		return shareRmCmd(dir, args[1:])
	}
	return shareCmd(dir, args)
}

// parseFileArgs parses flags placed before or after a single file argument.
func parseFileArgs(fs *flag.FlagSet, args []string) (string, error) {
	if err := fs.Parse(args); err != nil {
		return "", err
	}
	if fs.NArg() == 0 {
		return "", errors.New("needs a file (see mdv share --help)")
	}
	arg := fs.Arg(0)
	if err := fs.Parse(fs.Args()[1:]); err != nil {
		return "", err
	}
	if fs.NArg() > 0 {
		return "", errors.New("takes exactly one file")
	}
	return arg, nil
}

func shareStatusCmd(dir string) error {
	state, err := loadShareState(dir)
	if err != nil {
		return err
	}
	if len(state) == 0 {
		fmt.Println("No shared docs yet (mdv share <file.md>).")
		return nil
	}
	rels := make([]string, 0, len(state))
	for rel := range state {
		rels = append(rels, rel)
	}
	sort.Strings(rels)
	for _, rel := range rels {
		e := state[rel]
		status := "✓ shared"
		if b, err := os.ReadFile(filepath.Join(dir, rel)); err != nil {
			status = "✗ missing"
		} else if docHash(dir, filepath.Join(dir, rel), b) != e.SHA256 {
			status = "● changed"
		}
		fmt.Printf("%-10s %s  %s\n", status, rel, dimStyle.Render(link(e.URL)))
	}
	return nil
}

func shareCmd(dir string, args []string) error {
	fs := flag.NewFlagSet("share", flag.ContinueOnError)
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	arg, err := parseFileArgs(fs, args)
	if err != nil {
		return fmt.Errorf("share %w", err)
	}
	path, rel, err := resolveDoc(dir, arg)
	if err != nil {
		return err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	images, err := localImages(dir, path, content)
	if err != nil {
		return err
	}
	state, err := loadShareState(dir)
	if err != nil {
		return err
	}
	hash := docHash(dir, path, content)
	title := docTitle(rel, content)

	e, exists := state[rel]
	if exists && e.SHA256 == hash {
		fmt.Println("Unchanged:", link(e.URL))
		return nil
	}

	var newImages []string
	for _, image := range images {
		if _, ok := e.Images[image.sha]; !ok {
			newImages = append(newImages, image.ref)
		}
	}
	summary := ""
	if len(newImages) > 0 {
		summary = fmt.Sprintf(" (uploading %d image(s): %s)", len(newImages), strings.Join(newImages, ", "))
	}
	if exists {
		if err := confirm(fmt.Sprintf("Update the shared gist %s with %s%s?", link(e.URL), rel, summary), *yes); err != nil {
			return err
		}
	} else {
		if err := confirm(fmt.Sprintf("Share %s as a secret gist (anyone with the link can read it)%s?", rel, summary), *yes); err != nil {
			return err
		}
		// Create the gist first: its ID is needed for the image URLs. It starts
		// with image placeholders and gets the real links in the push below.
		if e, err = createGist(filepath.Base(rel), title, withPlaceholders(content)); err != nil {
			return err
		}
		// Record it right away so a failure below retries as an update
		// instead of creating a second gist.
		state[rel] = e
		if err := state.save(dir); err != nil {
			return err
		}
	}

	if e.Title != title {
		if _, err := gh(nil, "gist", "edit", e.GistID, "--desc", title); err != nil {
			fmt.Fprintln(os.Stderr, "mdv: could not update the gist description:", err)
		} else {
			e.Title = title
		}
	}
	files, err := pushToGist(e, content, images)
	if err != nil {
		state[rel] = e
		_ = state.save(dir)
		return fmt.Errorf("%s: %w; run mdv share again to retry", e.URL, err)
	}
	e.SHA256 = hash
	e.SharedAt = time.Now().UTC().Format(time.RFC3339)
	e.Images = files
	state[rel] = e
	if err := state.save(dir); err != nil {
		return err
	}
	if exists {
		fmt.Println("Updated:", link(e.URL))
	} else {
		fmt.Println("Shared:", link(e.URL))
	}
	return nil
}

// docTitle is the doc's H1, or a title made from its file name.
func docTitle(rel string, content []byte) string {
	icon, title, _ := splitTitle(content)
	if t := strings.TrimSpace(icon + " " + title); t != "" {
		return t
	}
	return titleFromFile(rel)
}

func createGist(file, title string, content []byte) (shareEntry, error) {
	out, err := gh(content, "gist", "create", "--filename", file, "--desc", title, "-")
	if err != nil {
		return shareEntry{}, err
	}
	url := strings.TrimSpace(string(out))
	id := url[strings.LastIndex(url, "/")+1:]
	if id == "" {
		return shareEntry{}, fmt.Errorf("unexpected gh gist create output %q", url)
	}
	var g struct {
		HTMLURL string `json:"html_url"`
		Owner   struct {
			Login string `json:"login"`
		} `json:"owner"`
	}
	meta, err := gh(nil, "api", "gists/"+id)
	if err != nil {
		return shareEntry{}, err
	}
	if err := json.Unmarshal(meta, &g); err != nil {
		return shareEntry{}, err
	}
	return shareEntry{GistID: id, URL: g.HTMLURL, Owner: g.Owner.Login, File: file, Title: title}, nil
}

// pushToGist clones the gist, writes the doc with its local images linked to
// their raw gist URLs, adds the images, drops images the doc no longer uses
// and pushes. It returns the image files now in the gist, by content hash.
func pushToGist(e shareEntry, content []byte, images []localImage) (map[string]string, error) {
	tmp, err := os.MkdirTemp("", "mdv-share-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	repo := filepath.Join(tmp, "gist")
	// HTTPS with gh as the credential helper: works without SSH keys or
	// known_hosts entries for gist.github.com.
	if _, err := git(tmp, append(ghCredentials, "clone", "--quiet", "https://gist.github.com/"+e.GistID+".git", repo)...); err != nil {
		return nil, err
	}

	files := map[string]string{}
	urls := map[string]string{}
	for _, image := range images {
		// "zz-" sorts images after the doc, so the gist page shows the doc first.
		name := "zz-image-" + image.sha[:12] + strings.ToLower(filepath.Ext(image.path))
		files[image.sha] = name
		urls[image.ref] = fmt.Sprintf("https://gist.githubusercontent.com/%s/%s/raw/%s", e.Owner, e.GistID, name)
		if _, err := os.Stat(filepath.Join(repo, name)); err == nil {
			continue
		}
		b, err := os.ReadFile(image.path)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(filepath.Join(repo, name), b, 0o644); err != nil {
			return nil, err
		}
	}
	// Remove images the doc no longer references.
	entries, err := os.ReadDir(repo)
	if err != nil {
		return nil, err
	}
	keep := map[string]bool{e.File: true}
	for _, name := range files {
		keep[name] = true
	}
	for _, d := range entries {
		if !d.IsDir() && !keep[d.Name()] {
			if err := os.Remove(filepath.Join(repo, d.Name())); err != nil {
				return nil, err
			}
		}
	}
	if err := os.WriteFile(filepath.Join(repo, e.File), withRawURLs(content, urls), 0o644); err != nil {
		return nil, err
	}

	if _, err := git(repo, "add", "-A"); err != nil {
		return nil, err
	}
	if out, err := git(repo, "status", "--porcelain"); err != nil {
		return nil, err
	} else if len(bytes.TrimSpace(out)) == 0 {
		return files, nil
	}
	if _, err := git(repo, "commit", "--quiet", "-m", "mdv share"); err != nil {
		return nil, err
	}
	if _, err := git(repo, append(ghCredentials, "push", "--quiet")...); err != nil {
		return nil, err
	}
	return files, nil
}

func shareRmCmd(dir string, args []string) error {
	fs := flag.NewFlagSet("rm", flag.ContinueOnError)
	yes := fs.Bool("yes", false, "skip the confirmation prompt")
	arg, err := parseFileArgs(fs, args)
	if err != nil {
		return fmt.Errorf("share rm %w", err)
	}
	// The doc itself may be gone already, so look it up by relative path.
	rel := arg
	if filepath.IsAbs(arg) || strings.HasPrefix(arg, "~") {
		p, err := expandPath(arg)
		if err != nil {
			return err
		}
		if rel, err = filepath.Rel(dir, p); err != nil {
			return err
		}
	}
	rel = filepath.Clean(rel)
	state, err := loadShareState(dir)
	if err != nil {
		return err
	}
	e, ok := state[rel]
	if !ok {
		return fmt.Errorf("%s is not shared (see mdv share status)", rel)
	}
	if err := confirm(fmt.Sprintf("Delete the gist %s for %s? The link stops working for everyone.", link(e.URL), rel), *yes); err != nil {
		return err
	}
	if _, err := gh(nil, "gist", "delete", e.GistID, "--yes"); err != nil {
		if !strings.Contains(strings.ToLower(err.Error()), "not found") && !strings.Contains(err.Error(), "404") {
			return err
		}
		fmt.Println(dimStyle.Render("The gist was already gone."))
	}
	delete(state, rel)
	if err := state.save(dir); err != nil {
		return err
	}
	fmt.Println("Deleted the share for", rel)
	return nil
}

// ghCredentials makes git authenticate with gh's token (and only gh's).
var ghCredentials = []string{"-c", "credential.helper=", "-c", "credential.helper=!gh auth git-credential"}

func gh(stdin []byte, args ...string) ([]byte, error) {
	return run("", stdin, "gh", args...)
}

func git(dir string, args ...string) ([]byte, error) {
	return run(dir, nil, "git", args...)
}

func run(dir string, stdin []byte, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		label := name + " " + strings.Join(args[:min(2, len(args))], " ")
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return nil, fmt.Errorf("%s: %s", label, msg)
		}
		return nil, fmt.Errorf("%s: %w", label, err)
	}
	return out, nil
}
