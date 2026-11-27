package share

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
)

// CheckAccess reports whether mdv share can work with the current settings.
func CheckAccess() (bool, string) {
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
