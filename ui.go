package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

type mode int

const (
	modeList mode = iota
	modeRead
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("62")).Padding(0, 1)
	dimStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
)

type fileItem struct {
	path    string
	rel     string
	modTime time.Time
}

func (f fileItem) Title() string       { return f.rel }
func (f fileItem) Description() string { return f.modTime.Format("2006-01-02 15:04") }
func (f fileItem) FilterValue() string { return f.rel }

type model struct {
	dir       string
	style     string
	singleton bool // opened with a path argument: no list to go back to

	mode   mode
	list   list.Model
	vp     viewport.Model
	file   string
	raw    string
	err    error
	width  int
	height int
	ready  bool
}

func newModel(dir, file, style string) model {
	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.Title = "mdv · " + tildify(dir)
	l.SetStatusBarItemName("file", "files")
	l.SetShowHelp(true)

	m := model{dir: dir, style: style, list: l, vp: viewport.New(0, 0)}
	if file != "" {
		m.singleton = true
		m.mode = modeRead
		m.file = file
	}
	return m
}

func (m model) Init() tea.Cmd {
	if m.singleton {
		return loadFile(m.file)
	}
	return scanDir(m.dir)
}

// --- messages / commands ---

type filesMsg []list.Item
type fileMsg struct {
	path, content string
	err           error
}

func scanDir(dir string) tea.Cmd {
	return func() tea.Msg {
		var items []fileItem
		_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() && p != dir && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			if d.IsDir() || !isMarkdown(p) {
				return nil
			}
			info, err := d.Info()
			if err != nil {
				return nil
			}
			rel, _ := filepath.Rel(dir, p)
			items = append(items, fileItem{path: p, rel: rel, modTime: info.ModTime()})
			return nil
		})
		sort.Slice(items, func(i, j int) bool { return items[i].modTime.After(items[j].modTime) })
		out := make([]list.Item, len(items))
		for i, it := range items {
			out[i] = it
		}
		return filesMsg(out)
	}
}

func loadFile(path string) tea.Cmd {
	return func() tea.Msg {
		b, err := os.ReadFile(path)
		return fileMsg{path: path, content: string(b), err: err}
	}
}

// --- update ---

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.ready = true
		m.list.SetSize(msg.Width, max(msg.Height-1, 1)) // leave a line for the hint
		m.vp.Width = msg.Width
		m.vp.Height = max(msg.Height-2, 1) // header + footer
		m.render()
		return m, nil

	case filesMsg:
		return m, m.list.SetItems(msg)

	case fileMsg:
		m.file, m.raw, m.err = msg.path, msg.content, msg.err
		m.render()
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}

	if m.mode == modeRead {
		return m.updateRead(msg)
	}
	return m.updateList(msg)
}

func (m model) updateList(msg tea.Msg) (tea.Model, tea.Cmd) {
	filtering := m.list.FilterState() == list.Filtering

	switch msg := msg.(type) {
	case tea.MouseMsg:
		if msg.Action == tea.MouseActionPress {
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				m.list.CursorUp()
				return m, nil
			case tea.MouseButtonWheelDown:
				m.list.CursorDown()
				return m, nil
			}
		}
	case tea.KeyMsg:
		if !filtering && msg.String() == "enter" {
			if it, ok := m.list.SelectedItem().(fileItem); ok {
				m.mode = modeRead
				m.file, m.raw, m.err = it.path, "", nil
				m.vp.SetContent("")
				return m, loadFile(it.path)
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) updateRead(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyMsg); ok {
		switch k.String() {
		case "q":
			return m, tea.Quit
		case "esc", "backspace":
			if m.singleton {
				return m, tea.Quit
			}
			m.mode = modeList
			return m, scanDir(m.dir)
		case "g", "home":
			m.vp.GotoTop()
			return m, nil
		case "G", "end":
			m.vp.GotoBottom()
			return m, nil
		case "r":
			return m, loadFile(m.file)
		}
	}

	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

// render re-renders the current file at the current width, keeping scroll position.
func (m *model) render() {
	if !m.ready || m.file == "" {
		return
	}
	if m.err != nil {
		m.vp.SetContent(errStyle.Render(m.err.Error()))
		return
	}
	wrap := min(m.width-2, 120)
	r, err := glamour.NewTermRenderer(glamour.WithStandardStyle(m.style), glamour.WithWordWrap(max(wrap, 20)))
	out := m.raw
	if err == nil {
		if s, err := r.Render(m.raw); err == nil {
			out = s
		}
	}
	offset := m.vp.YOffset
	m.vp.SetContent(out)
	m.vp.SetYOffset(offset)
}

// --- view ---

func (m model) View() string {
	if !m.ready {
		return ""
	}
	if m.mode == modeList {
		hint := ""
		if len(m.list.Items()) == 0 {
			hint = dimStyle.Render("  No files yet — add .md files to " + tildify(m.dir))
		}
		return m.list.View() + "\n" + hint
	}

	header := titleStyle.Render(filepath.Base(m.file)) + " " + dimStyle.Render(tildify(filepath.Dir(m.file)))
	back := "esc back"
	if m.singleton {
		back = "esc quit"
	}
	footer := dimStyle.Render(fmt.Sprintf(" %3.0f%%  ↑/↓ wheel scroll · space/b page · g/G top/bottom · r reload · %s · q quit", m.vp.ScrollPercent()*100, back))
	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().MaxWidth(m.width).Render(header),
		m.vp.View(),
		lipgloss.NewStyle().MaxWidth(m.width).Render(footer),
	)
}

func tildify(p string) string {
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(p, home) {
		return "~" + strings.TrimPrefix(p, home)
	}
	return p
}
