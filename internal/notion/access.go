package notion

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"

	"github.com/jakobluthman95/mdv/internal/term"
)

// CheckAccess reports whether mdv notion can work with the current settings.
// Notion is optional: without a token or the ntn CLI it is not set up.
func CheckAccess() (term.Access, string) {
	if notionUsesToken() {
		out, err := notionRequest(http.MethodGet, "v1/users/me", nil, nil)
		if err != nil {
			return term.AccessBroken, "NOTION_API_TOKEN: " + err.Error()
		}
		var me struct {
			Name string `json:"name"`
		}
		_ = json.Unmarshal(out, &me)
		return term.AccessOK, fmt.Sprintf("API token works (integration %q)", me.Name)
	}
	if _, err := exec.LookPath("ntn"); err != nil {
		return term.NotSetUp, "not set up (optional, only for mdv notion)"
	}
	if err := exec.Command("ntn", "whoami").Run(); err != nil {
		return term.AccessBroken, "ntn is installed but not logged in: run ntn login"
	}
	return term.AccessOK, "ntn CLI logged in"
}
