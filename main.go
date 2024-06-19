package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.design/x/clipboard"
)

const (
	primaryColor = lipgloss.Color("#6f8f92")
)

const DefaultPeriod = int64(30)

var globalStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.RoundedBorder()).
	BorderForeground(primaryColor).
	Foreground(primaryColor).
	PaddingRight(2)

type Application struct {
	Key           string
	Name          string
	Account       string
	Code          string
	RequiresTouch bool
	Period        int64
	TimeRemaining int64
}

// GetDurationLabel returns the unicode symbol representing time left until code expires
func (app *Application) GetDurationLabel() string {
	timeRemaining := app.Period - time.Now().Unix()%app.Period
	return fmt.Sprintf("(%02d)", timeRemaining)
}

type Model struct {
	Applications []Application
	Cursor       int
	Messages     map[string]string

	// Viewport size
	MaxWidth  int
	MaxHeight int

	// For scrollable list
	Start int
	End   int
}

// ShowMessage sets a message to show for given duration, stored using the current selected application as the key.
func (m *Model) ShowMessage(index int, text string, duration time.Duration) {
	app := m.Applications[index]
	key := fmt.Sprintf("%s:%s", app.Name, app.Account)
	m.Messages[key] = text

	// Clear message after designated time.
	go func() {
		time.Sleep(duration)
		m.Messages[key] = ""
	}()
}

// ShowMessage sets a message to show for given duration, stored using the current selected application as the key.
func (m *Model) ClearMessage(index int) {
	app := m.Applications[index]
	key := fmt.Sprintf("%s:%s", app.Name, app.Account)
	m.Messages[key] = ""
}

// RequestTouch uses `ykman` to prompt the user to touch their Yubikey
func (m *Model) RequestTouch() tea.Msg {
	serial := os.Getenv("YUBIKEY_SERIAL_NUMBER")

	app := m.Applications[m.Cursor]
	key := fmt.Sprintf("%s:%s", app.Name, app.Account)
	cmd := exec.Command("ykman", "--device", serial, "oath", "accounts", "code", key)

	buf, err := cmd.Output()
	if err != nil {
		return err.(errMsg)
	}

	// Waiting for user touch here...

	lines := bytes.Split(buf, []byte("\n"))

	if len(lines) < 2 {
		err := fmt.Errorf("Failed")
		return err.(errMsg)
	}

	// Parse the code, and return it as a message
	line := lines[0]
	matches := codeRegexp.FindSubmatch(line)
	code := matches[2]

	return codeMsg{
		Key:  []byte(key),
		Code: code,
	}
}

var codeRegexp = regexp.MustCompile("(.*\\S)\\s+(\\d+|\\[Requires Touch\\])")

// getCodes fetches list of applications w/codes using ykman
func (m *Model) getCodes() tea.Msg {
	serial := os.Getenv("YUBIKEY_SERIAL_NUMBER")

	cmd := exec.Command("ykman", "--device", serial, "oath", "accounts", "code")
	buf, err := cmd.Output()
	if err != nil {
		return err.(errMsg)
	}

	lines := bytes.Split(buf, []byte("\n"))

	applications := make([]Application, 0)

	for _, line := range lines {
		if len(line) == 0 || string(line) == "\n" {
			continue
		}

		matches := codeRegexp.FindSubmatch(line)
		info := bytes.Split(matches[1], []byte(":"))
		name := info[0]
		acc := make([]byte, 0)
		if len(info) > 1 {
			acc = info[1]
		}

		code := matches[2]

		// TODO: Get Period length for each account using `ykman oath accounts list -P`
		period := DefaultPeriod
		timeRemaining := period - time.Now().Unix()%period

		app := Application{
			Key:           string(matches[1]),
			Name:          string(name),
			Account:       string(acc),
			Code:          string(code),
			RequiresTouch: string(code) == "[Requires Touch]",
			Period:        period,
			TimeRemaining: timeRemaining,
		}

		// Compare to existing state before returning,
		// so we can retain any existing code if it hasn't cycled yet.
		for _, existingApp := range m.Applications {
			if existingApp.Key == app.Key && app.TimeRemaining < existingApp.TimeRemaining {
				// Keep the same code, since we haven't cycled to a new one yet.
				app.Code = existingApp.Code
				app.RequiresTouch = existingApp.RequiresTouch
			}
		}

		applications = append(applications, app)
	}

	return codesMsg{
		Applications: applications,
	}
}

type codesMsg struct {
	Applications []Application
}

type codeMsg struct {
	Key  []byte
	Code []byte
}

type errMsg error

func startTimer() tea.Msg {
	time.Sleep(time.Second * 1)
	return tickMsg{}
}

type tickMsg struct {
}

func (m Model) Init() tea.Cmd {
	return m.getCodes
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// Received a code after a touch event
	case codeMsg:
		key := string(msg.Key)
		code := string(msg.Code)

		for i, app := range m.Applications {
			if app.Key == key {
				m.Applications[i].Code = code
				m.Applications[i].RequiresTouch = false // Temporarily no longer requires touch

				// Silently copy to clipboard, so we can still show the code right away
				CopyToClipboard(code)
			}
		}

		m.ClearMessage(m.Cursor)
		return m, startTimer

	// Received full list of codes for all applications
	case codesMsg:
		m.Applications = msg.Applications
		m.Start = 0
		m.End = len(m.Applications) - 1
		return m, startTimer

	// Refetch codes
	case tickMsg:
		return m, m.getCodes

	case tea.WindowSizeMsg:
		h, v := globalStyle.GetFrameSize()
		m.MaxWidth = msg.Width - h
		m.MaxHeight = msg.Height - v

	case tea.KeyMsg:
		switch msg.String() {
		case "c", "enter":
			if m.Applications[m.Cursor].RequiresTouch {
				m.ShowMessage(m.Cursor, "[Touch Key]", time.Second*30)
				return m, m.RequestTouch
			} else {
				// Already have a code, so copy to clipboard
				CopyToClipboard(m.Applications[m.Cursor].Code)
				m.ShowMessage(m.Cursor, "Copied!", time.Second*3)
			}

			break

		case "ctrl+c", "q":
			return m, tea.Quit

		case "up", "k":
			m.Cursor--

			if m.Cursor < 0 {
				// Loop to end of list
				m.Cursor = len(m.Applications) - 1
			}

		case "down", "j":
			m.Cursor++

			if m.Cursor >= len(m.Applications) {
				// Loop around to top
				m.Cursor = 0
			}
		}
	}

	return m, nil
}

func (m Model) View() string {
	out := ""

	extraOffset := 4 // Magic number of spaces and extra chars
	codeWidth := 16
	durationWidth := 4

	labelWidth := m.MaxWidth - (codeWidth + durationWidth) - extraOffset

	for i, app := range m.Applications {
		if i > 0 {
			out += "\n"
		}

		cursor := "  "
		if m.Cursor == i {
			cursor = "> "
		}

		label := fmt.Sprintf("%s%s", cursor, app.Name)
		if len(label) > labelWidth && labelWidth > 3 {
			label = label[:labelWidth-3] + "..."
		}

		appName := lipgloss.NewStyle().
			Bold(m.Cursor == i).
			Width(labelWidth).
			Render(label)

		key := fmt.Sprintf("%s:%s", app.Name, app.Account)
		msg := app.Code

		if val, ok := m.Messages[key]; ok && val != "" {
			msg = m.Messages[key]
		}

		code := lipgloss.NewStyle().
			Align(lipgloss.Right).
			Width(codeWidth).
			Render(msg)

		duration := lipgloss.NewStyle().
			Align(lipgloss.Right).
			Width(durationWidth).
			Render(app.GetDurationLabel())

		out += lipgloss.JoinHorizontal(lipgloss.Bottom, appName, " ", code, " ", duration)
	}

	return globalStyle.Copy().
		Width(m.MaxWidth).
		Height(m.MaxHeight).
		Render(out)
}

func main() {
	err := clipboard.Init()
	if err != nil {
		panic(err)
	}

	m := Model{
		Messages: make(map[string]string),
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	// p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}
