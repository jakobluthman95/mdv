package gitx

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func GH(stdin []byte, args ...string) ([]byte, error) {
	return Run("", stdin, "gh", args...)
}

func Git(dir string, args ...string) ([]byte, error) {
	return Run(dir, nil, "git", args...)
}

func Run(dir string, stdin []byte, name string, args ...string) ([]byte, error) {
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

// Diff renders a git-style diff of old against new, labelled oldLabel/rel
// and newLabel/rel.
func Diff(oldLabel, newLabel, rel string, old, new []byte) (string, error) {
	tmp, err := os.MkdirTemp("", "mdv-diff-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmp)
	a, b := filepath.Join(oldLabel, rel), filepath.Join(newLabel, rel)
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

func Context() (repo, app, branch string) {
	top, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", "", ""
	}
	root := strings.TrimSpace(string(top))
	repo = filepath.Base(root)
	if cwd, err := os.Getwd(); err == nil {
		if real, err := filepath.EvalSymlinks(cwd); err == nil {
			cwd = real
		}
		if realRoot, err := filepath.EvalSymlinks(root); err == nil {
			root = realRoot
		}
		if rel, err := filepath.Rel(root, cwd); err == nil {
			parts := strings.Split(filepath.ToSlash(rel), "/")
			if len(parts) >= 2 && (parts[0] == "apps" || parts[0] == "packages") {
				app = parts[1]
			}
		}
	}
	if out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output(); err == nil {
		if b := strings.TrimSpace(string(out)); b != "HEAD" && b != "main" && b != "master" {
			branch = b
		}
	}
	return repo, app, branch
}
