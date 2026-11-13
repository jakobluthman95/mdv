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
	problems := false
	report := func(label string, good bool, detail string) {
		if good {
			ok(label, detail)
		} else {
			bad(label, detail)
			problems = true
		}
	}

	if _, err := os.Stat(configEnvFile()); err == nil {
		detail := tildify(configEnvFile())
		if envFileExplicit() {
			detail += " (from MDV_ENV_FILE)"
		}
		ok("config", detail)
	} else {
		info("config", tildify(configEnvFile())+" not found (optional; run mdv init to create it)")
	}

	if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
		ok("docs", fmt.Sprintf("%s (%d docs)", tildify(dir), len(markdownFiles(dir))))
	} else {
		report("docs", false, tildify(dir)+" does not exist yet (mdv creates it on first run)")
	}

	good, detail := checkNotion()
	report("notion", good, detail)

	if _, err := exec.LookPath("git"); err != nil {
		report("git", false, "git not found; mdv share needs it to upload images")
	} else {
		ok("git", "found")
	}
	good, detail = checkGitHub()
	report("github", good, detail)

	if problems {
		fmt.Println(dimStyle.Render("\nRun mdv init to set up tokens, or see the README for the CLI options."))
	}
	return nil
}

// checkNotion reports whether mdv notion can work with the current settings.
func checkNotion() (bool, string) {
	if notionUsesToken() {
		out, err := notionRequest(http.MethodGet, "v1/users/me", nil, nil)
		if err != nil {
			return false, "NOTION_API_TOKEN: " + err.Error()
		}
		var me struct {
			Name string `json:"name"`
		}
		_ = json.Unmarshal(out, &me)
		return true, fmt.Sprintf("API token works (integration %q)", me.Name)
	}
	if _, err := exec.LookPath("ntn"); err != nil {
		return false, "no NOTION_API_TOKEN and no ntn CLI; mdv notion won't work (browsing still does)"
	}
	if err := exec.Command("ntn", "whoami").Run(); err != nil {
		return false, "ntn is installed but not logged in: run ntn login"
	}
	return true, "ntn CLI logged in"
}

// checkGitHub reports whether mdv share can work with the current settings.
func checkGitHub() (bool, string) {
	if githubToken() != "" {
		out, err := githubAPI(http.MethodGet, "user", nil)
		if err != nil {
			return false, "GITHUB_TOKEN: " + err.Error()
		}
		var u struct {
			Login string `json:"login"`
		}
		_ = json.Unmarshal(out, &u)
		return true, fmt.Sprintf("GITHUB_TOKEN works (%s); it needs gist read/write access", u.Login)
	}
	if _, err := exec.LookPath("gh"); err != nil {
		return false, "no GITHUB_TOKEN and no gh CLI; mdv share won't work"
	}
	out, err := exec.Command("gh", "auth", "status").CombinedOutput()
	if err != nil {
		return false, "gh is installed but not logged in: run gh auth login"
	}
	if !strings.Contains(string(out), "'gist'") {
		return false, "gh is logged in without the gist scope: run gh auth refresh -s gist"
	}
	return true, "gh CLI logged in with the gist scope"
}
