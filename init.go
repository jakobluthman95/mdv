package main

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/term"
)

// envTemplate is .env.example, built into the binary so `mdv init` works
// without a clone of the repo.
//
//go:embed .env.example
var envTemplate string

// initKeys are the settings mdv init asks about, in order.
var initKeys = []string{"MDV_DIR", "NOTION_API_TOKEN", "GITHUB_TOKEN"}

func initCmd(args []string) error {
	path := configEnvFile()
	if len(args) == 1 && (args[0] == "--edit" || args[0] == "-e") {
		return initEdit(path)
	}
	if len(args) > 0 {
		if args[0] == "-h" || args[0] == "--help" {
			fmt.Println(initUsage)
			return nil
		}
		return fmt.Errorf("unknown argument %q (see mdv init --help)", args[0])
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return errors.New("mdv init asks questions and needs a terminal; use mdv init --edit, or copy .env.example by hand")
	}

	current, order, err := parseEnvFile(path)
	exists := err == nil
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if current == nil {
		current = map[string]string{}
	}
	// Values in the environment that differ from the file come from the shell
	// and take priority over whatever is written here.
	fromShell := map[string]bool{}
	for _, key := range initKeys {
		if v, ok := os.LookupEnv(key); ok && v != current[key] {
			fromShell[key] = true
		}
	}

	if exists {
		fmt.Printf("Updating %s. Press enter to keep a value.\n\n", tildify(path))
	} else {
		fmt.Printf("Creating %s. Every question can be skipped.\n\n", tildify(path))
	}
	values := map[string]string{}
	for k, v := range current {
		values[k] = v
	}

	// Docs folder
	def := current["MDV_DIR"]
	if def == "" {
		def = "~/dev/md"
	}
	fmt.Printf("Docs folder [%s]: ", def)
	if dir := readLine(); dir != "" {
		values["MDV_DIR"] = dir
	} else if current["MDV_DIR"] != "" {
		values["MDV_DIR"] = current["MDV_DIR"]
	}
	if fromShell["MDV_DIR"] {
		fmt.Println(dimStyle.Render("  note: MDV_DIR is also set in your shell, which takes priority over the file"))
	}

	askToken(values, current, fromShell, "NOTION_API_TOKEN",
		"Notion: paste an integration token (https://www.notion.so/profile/integrations)",
		"use the ntn CLI instead", checkNotion)
	askToken(values, current, fromShell, "GITHUB_TOKEN",
		"GitHub: paste a token with gist read/write access (https://github.com/settings/personal-access-tokens)",
		"use the gh CLI instead", checkGitHub)

	if exists && sameValues(current, values) {
		fmt.Println("\nNothing changed.")
		return nil
	}
	fmt.Printf("\nWrite %s? [Y/n] ", tildify(path))
	if a := strings.ToLower(readLine()); a != "" && a != "y" && a != "yes" {
		return errors.New("cancelled, nothing was written")
	}
	if err := writeEnvFile(path, values, order, exists); err != nil {
		return err
	}
	fmt.Printf("Wrote %s (only readable by you).\n", tildify(path))
	if dir, err := expandPath(values["MDV_DIR"]); err == nil && values["MDV_DIR"] != "" {
		_ = ensureDocsDir(dir)
	}
	fmt.Println(dimStyle.Render(`
Next:
  mdv doctor     check everything
  pnpx skills add git@github.com:jakobluthman95/mdv.git --skill mdv-docs --global
                 teach your AI agent to use mdv`))
	return nil
}

const initUsage = `Usage:
  mdv init          set up ~/.config/mdv/.env interactively (docs folder,
                    Notion token, GitHub token); tokens are checked right away
  mdv init --edit   open the settings file in $EDITOR (created from the
                    built-in template if it doesn't exist)

Every question can be skipped: without tokens, mdv uses the ntn and gh CLIs.
Running it again shows the current values and lets you change them.`

// askToken asks for one token, hidden, and checks it right away.
func askToken(values, current map[string]string, fromShell map[string]bool, key, intro, fallback string, check func() (bool, string)) {
	fmt.Println("\n" + intro)
	for {
		if cur := current[key]; cur != "" {
			fmt.Printf("  %s [keep %s] (enter = keep, - = remove): ", key, maskToken(cur))
		} else {
			fmt.Printf("  %s (enter = %s): ", key, fallback)
		}
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			fmt.Println("  could not read input:", err)
			return
		}
		input := strings.TrimSpace(string(b))
		switch {
		case input == "" && current[key] != "":
			values[key] = current[key]
		case input == "" || input == "-":
			delete(values, key)
		default:
			values[key] = input
		}

		// Check with exactly what will be written.
		if v, ok := values[key]; ok {
			os.Setenv(key, v)
		} else {
			os.Unsetenv(key)
		}
		good, detail := check()
		if good {
			fmt.Println("  ✓ " + detail)
		} else {
			fmt.Println("  ✗ " + detail)
		}
		if fromShell[key] {
			fmt.Println(dimStyle.Render("  note: " + key + " is also set in your shell, which takes priority over the file"))
		}
		if good || values[key] == "" {
			return
		}
		fmt.Print("  Try another token? n keeps this one anyway [Y/n] ")
		if a := strings.ToLower(readLine()); a == "n" || a == "no" {
			return
		}
	}
}

func readLine() string {
	s, _ := stdin.ReadString('\n')
	return strings.TrimSpace(s)
}

// maskToken shows a token's prefix and last four characters.
func maskToken(t string) string {
	if len(t) <= 8 {
		return "••••"
	}
	prefix := ""
	if i := strings.Index(t, "_"); i > 0 && i < 12 {
		prefix = t[:i+1]
	}
	return prefix + "…" + t[len(t)-4:]
}

func sameValues(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// writeEnvFile renders the built-in template with the values filled in.
// Settings mdv doesn't know are kept at the end. An existing file is backed
// up to <file>.bak first, since its comments are replaced by the template's.
func writeEnvFile(path string, values map[string]string, order []string, exists bool) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if exists {
		old, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.WriteFile(path+".bak", old, 0o600); err != nil {
			return err
		}
		fmt.Printf("Backed up the old file to %s.\n", tildify(path+".bak"))
	}
	content := renderEnv(values, order)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

var templateSetting = regexp.MustCompile(`(?m)^# ([A-Z_]+)=.*$`)

func renderEnv(values map[string]string, order []string) string {
	known := map[string]bool{}
	out := templateSetting.ReplaceAllStringFunc(envTemplate, func(line string) string {
		key := templateSetting.FindStringSubmatch(line)[1]
		known[key] = true
		if v, ok := values[key]; ok && v != "" {
			return key + "=" + quoteEnv(v)
		}
		return line
	})
	out = "# Written by mdv init on " + time.Now().Format("2006-01-02") + ".\n" + out
	var extra []string
	for _, key := range order {
		if !known[key] {
			if v, ok := values[key]; ok {
				extra = append(extra, key+"="+quoteEnv(v))
			}
		}
	}
	if len(extra) > 0 {
		out = strings.TrimRight(out, "\n") + "\n\n# --- Other settings (kept from the previous file) ---\n" + strings.Join(extra, "\n") + "\n"
	}
	return out
}

func quoteEnv(v string) string {
	switch {
	case !strings.ContainsAny(v, " #\"'\t"):
		return v
	case strings.Contains(v, `"`):
		return "'" + v + "'"
	}
	return `"` + v + `"`
}

// initEdit opens the settings file in the user's editor, creating it from
// the template first if needed.
func initEdit(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(envTemplate), 0o600); err != nil {
			return err
		}
		fmt.Printf("Created %s from the built-in template.\n", tildify(path))
	}
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}
	parts := strings.Fields(editor)
	cmd := exec.Command(parts[0], append(parts[1:], path)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}
