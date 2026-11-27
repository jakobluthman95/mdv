package term

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var DimStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

var Stdin = bufio.NewReader(os.Stdin)

func IsTerminal() bool {
	fi, err := os.Stdin.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

// AskTitle offers the suggested title and lets the user type another one.
// Every new page gets a title.
func AskTitle(suggested string, yes bool) (string, error) {
	if !yes && IsTerminal() {
		fmt.Printf("Title [%s]: ", suggested)
		answer, _ := Stdin.ReadString('\n')
		if a := strings.TrimSpace(answer); a != "" {
			return a, nil
		}
	}
	if suggested == "" {
		return "", errors.New("new pages need a title; pass --title")
	}
	return suggested, nil
}

func Confirm(question string, yes bool) error {
	if yes {
		return nil
	}
	if !IsTerminal() {
		return errors.New("no terminal to confirm on; pass --yes to skip the prompt")
	}
	fmt.Print(question + " [y/N] ")
	answer, _ := Stdin.ReadString('\n')
	if a := strings.ToLower(strings.TrimSpace(answer)); a != "y" && a != "yes" {
		return errors.New("cancelled, nothing was written")
	}
	return nil
}

// Link makes url clickable (OSC 8) when stdout is a terminal.
func Link(url string) string {
	if fi, err := os.Stdout.Stat(); err != nil || fi.Mode()&os.ModeCharDevice == 0 {
		return url
	}
	return "\x1b]8;;" + url + "\x1b\\" + url + "\x1b]8;;\x1b\\"
}

func Tildify(p string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(p, home) {
		return "~" + strings.TrimPrefix(p, home)
	}
	return p
}

// ParseFileArgs parses flags placed before or after a single file argument.
func ParseFileArgs(fs *flag.FlagSet, args []string) (string, error) {
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

func PrintJSON(v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}
