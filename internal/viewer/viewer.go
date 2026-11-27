package viewer

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jakobluthman95/mdv/internal/docs"
	"github.com/jakobluthman95/mdv/internal/notion"
	"github.com/jakobluthman95/mdv/internal/render"
	"github.com/jakobluthman95/mdv/internal/search"
	"github.com/jakobluthman95/mdv/internal/term"
)

type mode int

const (
	modeList mode = iota
	modeRead
)

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("62")).Padding(0, 1)
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
)

type fileItem struct {
	path    string
	rel     string
	modTime time.Time
	notion  notion.SyncState
	meta    docs.Meta
}

func (f fileItem) Title() string { return f.rel }
func (f fileItem) Description() string {
	d := f.modTime.Format("2006-01-02 15:04") + " · " + f.notion.String()
	for _, s := range []string{f.meta.Ticket, f.meta.Status} {
		if s != "" {
			d += " · " + s
		}
	}
	return d
}
func (f fileItem) FilterValue() string { return f.rel }

type model struct {
	dir       string
	style     string
	singleton bool // opened with a path argument: no list to go back to

	mode     mode
	list     list.Model
	vp       viewport.Model
	file     string
	raw      string
	warnings []string
	err      error
	width    int
	height   int
	ready    bool
}

func newModel(dir, file, style string) model {
	l := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	l.Title = "mdv · " + term.Tildify(dir)
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
		return loadFile(m.dir, m.file)
	}
	return scanDir(m.dir)
}

// --- messages / commands ---

type filesMsg []list.Item
type fileMsg struct {
	path, content string
	warnings      []string
	err           error
}

func scanDir(dir string) tea.Cmd {
	return func() tea.Msg {
		var items []fileItem
		for _, f := range docs.MarkdownFiles(dir) {
			items = append(items, fileItem{path: f.Path, rel: f.Rel, modTime: f.ModTime})
		}
		if state, err := notion.LoadState(dir); err == nil {
			for i := range items {
				items[i].notion = state.Status(dir, items[i].rel, items[i].path)
			}
		}
		for i := range items {
			if b, err := os.ReadFile(items[i].path); err == nil {
				items[i].meta, _, _, _ = docs.SplitFrontmatter(b)
			}
		}
		sort.Slice(items, func(i, j int) bool { return items[i].modTime.After(items[j].modTime) })
		out := make([]list.Item, len(items))
		for i, it := range items {
			out[i] = it
		}
		return filesMsg(out)
	}
}

func loadFile(dir, path string) tea.Cmd {
	return func() tea.Msg {
		b, err := os.ReadFile(path)
		if err != nil {
			return fileMsg{path: path, err: err}
		}
		var warnings []string
		if meta, _, ok, ferr := docs.SplitFrontmatter(b); ok && ferr == nil && len(meta.BuildsOn) > 0 {
			for _, u := range search.RegressedUpstream(search.LoadDocs(dir), search.Doc{AbsPath: path, Meta: meta}) {
				warnings = append(warnings, u.Message)
			}
		}
		return fileMsg{path: path, content: string(b), warnings: warnings}
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
		m.file, m.raw, m.warnings, m.err = msg.path, msg.content, msg.warnings, msg.err
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
				m.file, m.raw, m.warnings, m.err = it.path, "", nil, nil
				m.vp.SetContent("")
				return m, loadFile(m.dir, it.path)
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
			return m, loadFile(m.dir, m.file)
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
	out := m.raw
	if s, err := render.ForTerminal(m.raw, m.style, min(m.width-2, 120), m.warnings...); err == nil {
		out = s
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
			hint = term.DimStyle.Render("  No files yet — add .md files to " + term.Tildify(m.dir))
		}
		return m.list.View() + "\n" + hint
	}

	header := titleStyle.Render(filepath.Base(m.file)) + " " + term.DimStyle.Render(term.Tildify(filepath.Dir(m.file)))
	back := "esc back"
	if m.singleton {
		back = "esc quit"
	}
	footer := term.DimStyle.Render(fmt.Sprintf(" %3.0f%%  ↑/↓ wheel scroll · space/b page · g/G top/bottom · r reload · %s · q quit", m.vp.ScrollPercent()*100, back))
	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().MaxWidth(m.width).Render(header),
		m.vp.View(),
		lipgloss.NewStyle().MaxWidth(m.width).Render(footer),
	)
}

func Run(dir, file string) error {
	style := "light"
	if lipgloss.HasDarkBackground() {
		style = "dark"
	}

	m := newModel(dir, file, style)
	_, err := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	return err
}
