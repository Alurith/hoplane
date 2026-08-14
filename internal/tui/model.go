package tui

import (
	"context"
	"io"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Alurith/hoplane/internal/domain"
)

type model struct {
	list        list.Model
	connections []domain.Connection
	selected    *domain.Connection
	quitting    bool
}

func NewModel(connections []domain.Connection) model {
	items := make([]list.Item, 0, len(connections))
	for _, connection := range connections {
		items = append(items, NewItem(connection))
	}

	delegate := list.NewDefaultDelegate()
	delegate.SetHeight(3)
	delegate.SetSpacing(1)

	component := list.New(items, delegate, 0, 0)
	component.Title = "Hoplane connections"
	component.SetStatusBarItemName("connection", "connections")
	component.Styles.Title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("170"))

	return model{
		list:        component,
		connections: append([]domain.Connection(nil), connections...),
	}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch message := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(message.Width, message.Height)
		return m, nil

	case tea.KeyPressMsg:
		if !m.list.SettingFilter() {
			switch message.String() {
			case "q", "ctrl+c":
				m.quitting = true
				return m, tea.Quit
			case "enter":
				item, ok := m.list.SelectedItem().(Item)
				if ok {
					selected := item.Connection()
					m.selected = &selected
				}
				return m, tea.Quit
			}
		}
	}

	var command tea.Cmd
	m.list, command = m.list.Update(msg)
	return m, command
}

func (m model) View() tea.View {
	if m.quitting || m.selected != nil {
		view := tea.NewView("")
		view.AltScreen = true
		return view
	}
	view := tea.NewView(m.list.View())
	view.AltScreen = true
	return view
}

func (m model) Selected() (domain.Connection, bool) {
	if m.selected == nil {
		return domain.Connection{}, false
	}
	return *m.selected, true
}

func Pick(ctx context.Context, connections []domain.Connection, input io.Reader, output io.Writer) (domain.Connection, bool, error) {
	program := tea.NewProgram(
		NewModel(connections),
		tea.WithContext(ctx),
		tea.WithInput(input),
		tea.WithOutput(output),
	)
	finalModel, err := program.Run()
	if err != nil {
		return domain.Connection{}, false, err
	}
	result, ok := finalModel.(model)
	if !ok {
		return domain.Connection{}, false, nil
	}
	connection, selected := result.Selected()
	return connection, selected, nil
}
