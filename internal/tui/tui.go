package tui

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"switchbrd/internal/app"
	"switchbrd/internal/control"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type mode int

const (
	modeList mode = iota
	modeAdd
	modeRename
	modeConfirmRemove
)

type tickMsg time.Time

type refreshMsg struct {
	status control.StatusData
	apps   []app.App
	err    error
}

type actionResultMsg struct {
	err    error
	quit   bool
	detach bool
}

type model struct {
	client      *control.Client
	ownedServer bool
	detached    bool

	status    control.StatusData
	apps      []app.App
	selected  int
	mode      mode
	portInput textinput.Model
	nameInput textinput.Model
	errLine   string
	ready     bool
	width     int
	height    int
}

var (
	appShellStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("230")).
			Background(lipgloss.Color("24")).
			Padding(0, 1)
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1)
	sectionTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("222"))
	activeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			Bold(true)
	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("238")).
			Bold(true)
	metaLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("109"))
	helpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250"))
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("203")).
			Bold(true)
)

func Run(client *control.Client, ownedServer bool) error {
	portInput := textinput.New()
	portInput.Placeholder = "5175"
	portInput.Prompt = "port: "

	nameInput := textinput.New()
	nameInput.Placeholder = "optional name"
	nameInput.Prompt = "name: "

	m := model{
		client:      client,
		ownedServer: ownedServer,
		portInput:   portInput,
		nameInput:   nameInput,
	}

	_, err := tea.NewProgram(m).Run()
	return err
}

func (m model) Init() tea.Cmd {
	return tea.Batch(refreshCmd(m.client), tickCmd())
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.ready = true
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tickMsg:
		return m, tea.Batch(refreshCmd(m.client), tickCmd())
	case refreshMsg:
		if msg.err != nil {
			m.errLine = msg.err.Error()
			if m.ready {
				return m, tea.Quit
			}
			return m, nil
		}
		m.status = msg.status
		m.apps = sortedApps(msg.apps)
		if len(m.apps) == 0 {
			m.selected = 0
		} else if m.selected >= len(m.apps) {
			m.selected = len(m.apps) - 1
		}
		m.ready = true
		if m.errLine == "loading..." {
			m.errLine = ""
		}
		return m, nil
	case actionResultMsg:
		if msg.err != nil {
			m.errLine = msg.err.Error()
			return m, nil
		}
		m.errLine = ""
		if msg.detach {
			m.detached = true
		}
		if msg.quit {
			return m, tea.Quit
		}
		return m, refreshCmd(m.client)
	case tea.KeyMsg:
		switch m.mode {
		case modeAdd:
			return m.updateAddMode(msg)
		case modeRename:
			return m.updateRenameMode(msg)
		case modeConfirmRemove:
			return m.updateRemoveMode(msg)
		default:
			return m.updateListMode(msg)
		}
	}

	return m, nil
}

func (m model) View() string {
	if !m.ready {
		return appShellStyle.Render("loading...")
	}

	top := headerStyle.Render(" switchbrd ") + "\n" + m.renderStatusPanel()
	body := lipgloss.JoinHorizontal(lipgloss.Top, m.renderAppsPanel(), m.renderActionsPanel())

	return appShellStyle.
		Width(maxInt(m.width, 80)).
		Padding(0, 1, 1, 1).
		Render(lipgloss.JoinVertical(lipgloss.Left, top, body))
}

func (m model) renderStatusPanel() string {
	activeLine := "none"
	if m.status.Active != nil {
		activeLine = formatApp(*m.status.Active)
	}

	segments := []string{
		sectionTitleStyle.Render("Status"),
		fmt.Sprintf("%s %d", metaLabelStyle.Render("PID"), m.status.PID),
		fmt.Sprintf("%s %d", metaLabelStyle.Render("Apps"), m.status.AppCount),
		fmt.Sprintf("%s %s", metaLabelStyle.Render("Active"), activeLine),
	}
	if len(m.status.ProxyListenAddrs) > 0 {
		segments = append(segments, fmt.Sprintf("%s %s", metaLabelStyle.Render("Listen"), strings.Join(m.status.ProxyListenAddrs, ", ")))
	}

	return panelStyle.Width(maxInt(m.width-6, 78)).Render(strings.Join(segments, "   "))
}

func (m model) renderAppsPanel() string {
	lines := []string{sectionTitleStyle.Render("Apps")}
	if len(m.apps) == 0 {
		lines = append(lines, helpStyle.Render("No apps registered"))
	} else {
		for i, candidate := range m.apps {
			line := formatApp(candidate)
			prefix := "  "
			if m.status.Active != nil && m.status.Active.Name == candidate.Name {
				line = activeStyle.Render("ACTIVE " + line)
			}
			if i == m.selected {
				prefix = "› "
				line = selectedStyle.Render(line)
			}
			lines = append(lines, prefix+line)
		}
	}

	width := 42
	if m.width > 110 {
		width = 54
	}

	return panelStyle.Width(width).Height(12).Render(strings.Join(lines, "\n"))
}

func (m model) renderActionsPanel() string {
	var lines []string
	lines = append(lines, sectionTitleStyle.Render("Actions"))

	switch m.mode {
	case modeAdd:
		lines = append(lines,
			"Register a port",
			"",
			m.portInput.View(),
			m.nameInput.View(),
			"",
			helpStyle.Render("enter save"),
			helpStyle.Render("tab switch field"),
			helpStyle.Render("esc cancel"),
		)
	case modeRename:
		lines = append(lines,
			"Rename selected app",
			"",
			m.nameInput.View(),
			"",
			helpStyle.Render("enter save"),
			helpStyle.Render("esc cancel"),
		)
	case modeConfirmRemove:
		lines = append(lines,
			"Remove selected app?",
			"",
			errorStyle.Render("Press y to confirm"),
			helpStyle.Render("Any other key cancels"),
		)
	default:
		lines = append(lines,
			"enter  activate selected",
			"a      add app",
			"r      rename selected",
			"x      remove selected",
			"s      stop server",
			"d      detach session",
			"q      quit",
			"R      refresh",
			"j/k    move",
		)
	}
	if m.errLine != "" {
		lines = append(lines, "", errorStyle.Render(m.errLine))
	}

	return panelStyle.Width(36).Height(12).Render(strings.Join(lines, "\n"))
}

func (m model) updateListMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		if m.selected > 0 {
			m.selected--
		}
		return m, nil
	case "down", "j":
		if m.selected < len(m.apps)-1 {
			m.selected++
		}
		return m, nil
	case "enter":
		if selected, ok := m.selectedApp(); ok {
			return m, activateCmd(m.client, selected.Name)
		}
		return m, nil
	case "a":
		m.mode = modeAdd
		m.portInput.SetValue("")
		m.nameInput.SetValue("")
		m.portInput.Focus()
		m.nameInput.Blur()
		return m, textinput.Blink
	case "r":
		selected, ok := m.selectedApp()
		if !ok {
			return m, nil
		}
		m.mode = modeRename
		m.nameInput.SetValue(selected.Name)
		m.nameInput.Focus()
		return m, textinput.Blink
	case "x":
		if _, ok := m.selectedApp(); ok {
			m.mode = modeConfirmRemove
		}
		return m, nil
	case "s":
		return m, shutdownCmd(m.client, true, false)
	case "d":
		return m, actionResultCmd(nil, true, true)
	case "q":
		if m.ownedServer && !m.detached {
			return m, shutdownCmd(m.client, true, false)
		}
		return m, tea.Quit
	case "R":
		return m, refreshCmd(m.client)
	}

	return m, nil
}

func (m model) updateAddMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyEsc {
		m.mode = modeList
		m.portInput.Blur()
		m.nameInput.Blur()
		return m, nil
	}
	if msg.Type == tea.KeyTab || msg.Type == tea.KeyShiftTab {
		if m.portInput.Focused() {
			m.portInput.Blur()
			m.nameInput.Focus()
		} else {
			m.nameInput.Blur()
			m.portInput.Focus()
		}
		return m, nil
	}
	if msg.Type == tea.KeyEnter {
		port, err := strconv.Atoi(strings.TrimSpace(m.portInput.Value()))
		if err != nil {
			m.errLine = "invalid port"
			return m, nil
		}
		m.mode = modeList
		m.portInput.Blur()
		m.nameInput.Blur()
		return m, addCmd(m.client, port, strings.TrimSpace(m.nameInput.Value()))
	}

	var cmd tea.Cmd
	if m.portInput.Focused() {
		m.portInput, cmd = m.portInput.Update(msg)
	} else {
		m.nameInput, cmd = m.nameInput.Update(msg)
	}
	return m, cmd
}

func (m model) updateRenameMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	selected, ok := m.selectedApp()
	if !ok {
		m.mode = modeList
		return m, nil
	}
	if msg.Type == tea.KeyEsc {
		m.mode = modeList
		m.nameInput.Blur()
		return m, nil
	}
	if msg.Type == tea.KeyEnter {
		m.mode = modeList
		m.nameInput.Blur()
		return m, renameCmd(m.client, selected.Name, strings.TrimSpace(m.nameInput.Value()))
	}
	var cmd tea.Cmd
	m.nameInput, cmd = m.nameInput.Update(msg)
	return m, cmd
}

func (m model) updateRemoveMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	selected, ok := m.selectedApp()
	if !ok {
		m.mode = modeList
		return m, nil
	}
	m.mode = modeList
	if strings.EqualFold(msg.String(), "y") {
		return m, removeCmd(m.client, selected.Name)
	}
	return m, nil
}

func (m model) selectedApp() (app.App, bool) {
	if len(m.apps) == 0 || m.selected < 0 || m.selected >= len(m.apps) {
		return app.App{}, false
	}
	return m.apps[m.selected], true
}

func refreshCmd(client *control.Client) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		status, err := client.Status(ctx)
		if err != nil {
			return refreshMsg{err: err}
		}
		apps, _, err := client.List(ctx)
		if err != nil {
			return refreshMsg{err: err}
		}
		status.AppCount = len(apps)
		return refreshMsg{status: status, apps: apps}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func addCmd(client *control.Client, port int, name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if _, err := client.Add(ctx, port, name); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{}
	}
}

func renameCmd(client *control.Client, oldName string, newName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if _, err := client.Rename(ctx, oldName, newName); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{}
	}
}

func removeCmd(client *control.Client, name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := client.Remove(ctx, name); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{}
	}
}

func activateCmd(client *control.Client, target string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if _, err := client.Activate(ctx, target, ""); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{}
	}
}

func shutdownCmd(client *control.Client, quit bool, detach bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := client.Shutdown(ctx); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{quit: quit, detach: detach}
	}
}

func actionResultCmd(err error, quit bool, detach bool) tea.Cmd {
	return func() tea.Msg {
		return actionResultMsg{err: err, quit: quit, detach: detach}
	}
}

func sortedApps(apps []app.App) []app.App {
	cloned := append([]app.App(nil), apps...)
	sort.Slice(cloned, func(i, j int) bool {
		return cloned[i].Name < cloned[j].Name
	})
	return cloned
}

func formatApp(candidate app.App) string {
	return fmt.Sprintf("%s -> %d", candidate.Name, candidate.Port)
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
