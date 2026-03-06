package tui

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"dev-switchboard/internal/app"
	"dev-switchboard/internal/control"

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
	message string
	err     error
	quit    bool
	detach  bool
}

type model struct {
	client      *control.Client
	ownedServer bool
	detached    bool

	status     control.StatusData
	apps       []app.App
	selected   int
	mode       mode
	portInput  textinput.Model
	nameInput  textinput.Model
	statusLine string
	errLine    string
	ready      bool
}

var (
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	activeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
	helpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
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
		statusLine:  "loading...",
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
		m.statusLine = msg.message
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
		return "loading..."
	}

	var b strings.Builder
	b.WriteString(headerStyle.Render("dev-switchboard TUI"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("running | apps: %d | pid: %d", m.status.AppCount, m.status.PID))
	if m.status.Active == nil {
		b.WriteString(" | active: none")
	} else {
		b.WriteString(" | active: ")
		b.WriteString(formatApp(*m.status.Active))
	}
	b.WriteString("\n\n")

	if len(m.apps) == 0 {
		b.WriteString("no apps registered\n")
	} else {
		for i, candidate := range m.apps {
			marker := " "
			line := formatApp(candidate)
			if m.status.Active != nil && m.status.Active.Name == candidate.Name {
				marker = "*"
				line = activeStyle.Render(line)
			}
			if i == m.selected {
				b.WriteString("> ")
			} else {
				b.WriteString("  ")
			}
			b.WriteString(marker + " " + line)
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	switch m.mode {
	case modeAdd:
		b.WriteString("add app\n")
		b.WriteString(m.portInput.View())
		b.WriteString("\n")
		b.WriteString(m.nameInput.View())
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("enter save | tab switch field | esc cancel"))
	case modeRename:
		b.WriteString("rename app\n")
		b.WriteString(m.nameInput.View())
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("enter save | esc cancel"))
	case modeConfirmRemove:
		b.WriteString("remove selected app? (y/n)")
	default:
		b.WriteString(helpStyle.Render("j/k or arrows move | enter activate | a add | r rename | x remove | s stop | d detach | q quit | R refresh"))
	}

	if m.statusLine != "" {
		b.WriteString("\n")
		b.WriteString(m.statusLine)
	}
	if m.errLine != "" {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render(m.errLine))
	}

	return b.String()
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
		return m, shutdownCmd(m.client, true, false, "stopping server...")
	case "d":
		return m, actionResultCmd("detached", nil, true, true)
	case "q":
		if m.ownedServer && !m.detached {
			return m, shutdownCmd(m.client, true, false, "stopping owned server...")
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
		candidate, err := client.Add(ctx, port, name)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("added %s", formatApp(candidate))}
	}
}

func renameCmd(client *control.Client, oldName string, newName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		candidate, err := client.Rename(ctx, oldName, newName)
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("renamed %s", formatApp(candidate))}
	}
}

func removeCmd(client *control.Client, name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := client.Remove(ctx, name); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("removed %s", name)}
	}
}

func activateCmd(client *control.Client, target string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		candidate, err := client.Activate(ctx, target, "")
		if err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: fmt.Sprintf("active app: %s", formatApp(*candidate))}
	}
}

func shutdownCmd(client *control.Client, quit bool, detach bool, message string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := client.Shutdown(ctx); err != nil {
			return actionResultMsg{err: err}
		}
		return actionResultMsg{message: message, quit: quit, detach: detach}
	}
}

func actionResultCmd(message string, err error, quit bool, detach bool) tea.Cmd {
	return func() tea.Msg {
		return actionResultMsg{message: message, err: err, quit: quit, detach: detach}
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
