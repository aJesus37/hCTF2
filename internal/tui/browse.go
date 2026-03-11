package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Challenge is the data passed into the browser (subset of client.Challenge).
type Challenge struct {
	ID       string
	Title    string
	Category string
	Points   int
}

type BrowseModel struct {
	challenges []Challenge
	filtered   []Challenge
	cursor     int
	filter     string
	filtering  bool
	selected   *Challenge
}

func NewBrowseModel(challenges []Challenge) BrowseModel {
	return BrowseModel{challenges: challenges, filtered: challenges}
}

func (m BrowseModel) Init() tea.Cmd { return nil }

func (m BrowseModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.filtering {
			switch msg.String() {
			case "enter", "esc":
				m.filtering = false
			case "backspace":
				if len(m.filter) > 0 {
					m.filter = m.filter[:len(m.filter)-1]
					m.applyFilter()
				}
			default:
				if len(msg.String()) == 1 {
					m.filter += msg.String()
					m.applyFilter()
				}
			}
			return m, nil
		}
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "/":
			m.filtering = true
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.filtered) > 0 {
				ch := m.filtered[m.cursor]
				m.selected = &ch
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m *BrowseModel) applyFilter() {
	m.filtered = nil
	for _, ch := range m.challenges {
		if strings.Contains(strings.ToLower(ch.Title), strings.ToLower(m.filter)) ||
			strings.Contains(strings.ToLower(ch.Category), strings.ToLower(m.filter)) {
			m.filtered = append(m.filtered, ch)
		}
	}
	m.cursor = 0
}

func (m BrowseModel) View() string {
	var sb strings.Builder
	sb.WriteString(HeaderStyle.Render("Challenge Browser") + "\n")
	if m.filtering {
		sb.WriteString(fmt.Sprintf("Filter: %s█\n\n", m.filter))
	} else {
		sb.WriteString(MutedStyle.Render("↑/↓ navigate  / filter  enter select  q quit") + "\n\n")
	}
	for i, ch := range m.filtered {
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}
		title := ch.Title
		if len(title) > 28 {
			title = title[:25] + "..."
		}
		line := fmt.Sprintf("%s%-28s %-14s %4dpts", cursor, title, ch.Category, ch.Points)
		if i == m.cursor {
			sb.WriteString(lipgloss.NewStyle().Bold(true).Render(line) + "\n")
		} else {
			sb.WriteString(line + "\n")
		}
	}
	return sb.String()
}

// RunBrowser starts the interactive browser and returns the selected challenge ID, or "" if none selected.
func RunBrowser(challenges []Challenge) (string, error) {
	m := NewBrowseModel(challenges)
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return "", err
	}
	final := result.(BrowseModel)
	if final.selected != nil {
		return final.selected.ID, nil
	}
	return "", nil
}
