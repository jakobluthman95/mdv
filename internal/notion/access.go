package notion

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
)

// CheckAccess reports whether mdv notion can work with the current settings.
func CheckAccess() (bool, string) {
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
