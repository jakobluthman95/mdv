package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

// doctorCmd checks the setup without changing anything.
func doctorCmd(dir string) error {
	ok := func(label, detail string) { fmt.Printf("✓ %-8s %s\n", label, detail) }
	bad := func(label, detail string) { fmt.Printf("✗ %-8s %s\n", label, detail) }
	info := func(label, detail string) { fmt.Printf("· %-8s %s\n", label, detail) }

	if _, err := os.Stat(configEnvFile()); err == nil {
		ok("config", tildify(configEnvFile()))
	} else {
		info("config", tildify(configEnvFile())+" not found (optional; see .env.example)")
	}

	if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
		ok("docs", fmt.Sprintf("%s (%d docs)", tildify(dir), len(markdownFiles(dir))))
	} else {
		bad("docs", tildify(dir)+" does not exist yet (mdv creates it on first run)")
	}

	// Notion
	if notionUsesToken() {
		out, err := notionRequest(http.MethodGet, "v1/users/me", nil, nil)
		var me struct {
			Name string `json:"name"`
		}
		if err == nil {
			_ = json.Unmarshal(out, &me)
			ok("notion", fmt.Sprintf("API token works (integration %q)", me.Name))
		} else {
			bad("notion", "NOTION_API_TOKEN: "+err.Error())
		}
	} else if _, err := exec.LookPath("ntn"); err != nil {
		bad("notion", "no NOTION_API_TOKEN and no ntn CLI; mdv notion won't work (browsing still does)")
	} else if out, err := exec.Command("ntn", "whoami").CombinedOutput(); err != nil {
		bad("notion", "ntn is installed but not logged in: run ntn login")
	} else {
		_ = out
		ok("notion", "ntn CLI logged in")
	}

	// GitHub gists
	if _, err := exec.LookPath("git"); err != nil {
		bad("git", "git not found; mdv share needs it to upload images")
	} else {
		ok("git", "found")
	}
	if githubToken() != "" {
		out, err := githubAPI(http.MethodGet, "user", nil)
		var u struct {
			Login string `json:"login"`
		}
		if err == nil {
			_ = json.Unmarshal(out, &u)
			ok("github", fmt.Sprintf("GITHUB_TOKEN works (%s); it needs gist read/write access", u.Login))
		} else {
			bad("github", "GITHUB_TOKEN: "+err.Error())
		}
	} else if _, err := exec.LookPath("gh"); err != nil {
		bad("github", "no GITHUB_TOKEN and no gh CLI; mdv share won't work")
	} else if out, err := exec.Command("gh", "auth", "status").CombinedOutput(); err != nil {
		bad("github", "gh is installed but not logged in: run gh auth login")
	} else if !strings.Contains(string(out), "'gist'") {
		bad("github", "gh is logged in without the gist scope: run gh auth refresh -s gist")
	} else {
		ok("github", "gh CLI logged in with the gist scope")
	}
	return nil
}

func firstLine(b []byte) string {
	s, _, _ := strings.Cut(strings.TrimSpace(string(b)), "\n")
	return s
}
