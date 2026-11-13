package main

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// configEnvFile is mdv's optional settings file: $MDV_ENV_FILE when set
// (a ~ prefix is expanded), otherwise $XDG_CONFIG_HOME/mdv/.env, by default
// ~/.config/mdv/.env. Variables already set in the environment win over the
// file. MDV_ENV_FILE itself has to come from the environment, not a file.
func configEnvFile() string {
	if p := os.Getenv("MDV_ENV_FILE"); p != "" {
		if abs, err := expandPath(p); err == nil {
			return abs
		}
		return p
	}
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "mdv", ".env")
}

// envFileExplicit reports whether the settings file was chosen with
// MDV_ENV_FILE. An explicitly chosen file must exist.
func envFileExplicit() bool { return os.Getenv("MDV_ENV_FILE") != "" }

// loadEnvFile reads KEY=VALUE lines into the environment. Blank lines,
// # comments and an optional "export " prefix are allowed; values may be
// wrapped in single or double quotes.
//
// If the file doesn't exist, what happens depends on where the path came from:
//   - ~/.config/mdv/.env (the default): fine, mdv runs without settings.
//   - a path set with MDV_ENV_FILE: an error, because the user asked for
//     that file and probably mistyped the path.
func loadEnvFile(path string) error {
	values, _, err := parseEnvFile(path)
	if os.IsNotExist(err) && !envFileExplicit() {
		return nil
	}
	if err != nil {
		return err
	}
	for key, value := range values {
		if _, set := os.LookupEnv(key); !set {
			os.Setenv(key, value)
		}
	}
	return nil
}

// parseEnvFile reads a .env file without touching the environment. It returns
// the values and the keys in file order.
func parseEnvFile(path string) (map[string]string, []string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	values := map[string]string{}
	var keys []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1]
		} else if i := strings.Index(value, " #"); i >= 0 {
			value = strings.TrimSpace(value[:i])
		}
		if key == "" {
			continue
		}
		if _, seen := values[key]; !seen {
			keys = append(keys, key)
		}
		values[key] = value
	}
	return values, keys, sc.Err()
}
