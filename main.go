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
)

type Application struct {
	Name          string
	Account       string
	Code          string
	RequiresTouch bool
}

type Model struct {
	Applications []Application
	Cursor       int
}

var codeRegexp = regexp.MustCompile("(.*\\S)\\s+(\\d+|\\[Requires Touch\\])")

// getCodes fetches list of applications w/codes using ykman
func getCodes() tea.Msg {
	cmd := exec.Command("ykman", "oath", "accounts", "code")
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
		app := info[0]
		acc := info[1]
		code := matches[2]

		applications = append(applications, Application{
			Name:          string(app),
			Account:       string(acc),
			Code:          string(code),
			RequiresTouch: string(code) == "[Requires Touch]",
		})
	}

	return codesMsg{
		Applications: applications,
	}
}

type codesMsg struct {
	Applications []Application
}

type errMsg error

func startTimer() tea.Msg {
	duration := 30 - (time.Now().Unix() % 30)
	time.Sleep(time.Duration(duration))
	return tickMsg{}
}

type tickMsg struct {
}

func (m Model) Init() tea.Cmd {
	return getCodes
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case codesMsg:
		m.Applications = msg.Applications
		return m, startTimer

	case tickMsg:
		// Refetch codes
		return m, getCodes

	case tea.KeyMsg:
		switch msg.String() {
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

	for i, app := range m.Applications {
		cursor := "  "
		if m.Cursor == i {
			cursor = "> "
		}

		out += fmt.Sprintf("%s%s - %s\n", cursor, app.Name, app.Code)
	}

	return out
}

func main() {
	m := Model{}

	// p := tea.NewProgram(m, tea.WithAltScreen())
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}
