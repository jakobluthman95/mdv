package share

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strings"

	"github.com/jakobluthman95/mdv/internal/term"
)

// CheckAccess reports whether mdv share can work with the current settings.
// GitHub is optional: without a token or the gh CLI it is not set up.
func CheckAccess() (term.Access, string) {
	if githubToken() != "" {
		out, err := githubAPI(http.MethodGet, "user", nil)
		if err != nil {
			return term.AccessBroken, "GITHUB_TOKEN: " + err.Error()
		}
		var u struct {
			Login string `json:"login"`
		}
		_ = json.Unmarshal(out, &u)
		return term.AccessOK, fmt.Sprintf("GITHUB_TOKEN works (%s); it needs gist read/write access", u.Login)
	}
	if _, err := exec.LookPath("gh"); err != nil {
		return term.NotSetUp, "not set up (optional, only for mdv share)"
	}
	out, err := exec.Command("gh", "auth", "status").CombinedOutput()
	if err != nil {
		return term.AccessBroken, "gh is installed but not logged in: run gh auth login"
	}
	if !strings.Contains(string(out), "'gist'") {
		return term.AccessBroken, "gh is logged in without the gist scope: run gh auth refresh -s gist"
	}
	return term.AccessOK, "gh CLI logged in with the gist scope"
}
