package tui

import (
	"context"
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/Alurith/hoplane/internal/domain"
)

type screen uint8

const (
	screenList screen = iota
	screenConnectionForm
	screenDuplicateForm
	screenDeleteConfirm
	screenSaving
)

type listCommand uint8

const (
	commandNone listCommand = iota
	commandConnect
	commandAdd
	commandEdit
	commandDuplicate
	commandDeleteConfirm
)

type Action uint8

const (
	ActionNone Action = iota
	ActionConnect
)

type model struct {
	ctx    context.Context
	list   list.Model
	editor ConnectionEditor
	mode   screen

	form      connectionForm
	duplicate duplicateForm
	confirm   deleteConfirmation

	selected *domain.Connection
	action   Action
	status   error
	quitting bool

	pendingName     string
	pendingIndex    int
	pendingSelected bool
}

func NewModel(ctx context.Context, connections []domain.Connection, editor ConnectionEditor) model {
	if ctx == nil {
		ctx = context.Background()
	}
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
	component.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{connectKey, addKey, editKey, duplicateKey, deleteKey}
	}

	return model{
		ctx:    ctx,
		list:   component,
		editor: editor,
		mode:   screenList,
	}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch message := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(message.Width, message.Height)
		var command tea.Cmd
		switch m.mode {
		case screenConnectionForm:
			m.form, command = m.form.Update(message)
		case screenDuplicateForm:
			m.duplicate, command = m.duplicate.Update(message)
		case screenDeleteConfirm:
			m.confirm, command = m.confirm.Update(message)
		}
		return m, command
	case list.FilterMatchesMsg:
		var command tea.Cmd
		m.list, command = m.list.Update(message)
		m.applyPendingSelection()
		return m, command
	case mutationMsg:
		return m.updateMutation(message)
	case tea.KeyPressMsg:
		if key.Matches(message, quitKey) {
			m.quitting = true
			return m, tea.Quit
		}
		switch m.mode {
		case screenConnectionForm:
			if message.String() == "esc" {
				m.mode = screenList
				m.status = nil
				return m, nil
			}
			if key.Matches(message, skipKey) {
				form, command, skipped := m.form.SkipOptional()
				if skipped {
					m.form = form
					return m.submitConnectionForm(command)
				}
			}
			updated, command := m.updateConnectionForm(message)
			return updated, command
		case screenDuplicateForm:
			if message.String() == "esc" {
				m.mode = screenList
				m.status = nil
				return m, nil
			}
			updated, command := m.updateDuplicateForm(message)
			return updated, command
		case screenDeleteConfirm:
			if message.String() == "esc" {
				m.mode = screenList
				m.status = nil
				return m, nil
			}
			updated, command := m.updateDeleteConfirmation(message)
			return updated, command
		case screenSaving:
			return m, nil
		case screenList:
			if !m.list.SettingFilter() {
				switch {
				case key.Matches(message, enterKey), key.Matches(message, connectKey):
					updated, command := m.beginConnection()
					return updated, command
				case key.Matches(message, addKey):
					m.form = newAddForm()
					m.status = nil
					m.mode = screenConnectionForm
					return m, m.form.form.Init()
				case key.Matches(message, editKey):
					connection, ok := m.selectedItem()
					if !ok {
						return m, nil
					}
					if m.editor == nil || !m.editor.CanModify(connection) {
						m.status = fmt.Errorf("connection %q cannot be modified", connection.Name)
						return m, nil
					}
					m.form = newEditForm(connection)
					m.status = nil
					m.mode = screenConnectionForm
					return m, m.form.form.Init()
				case key.Matches(message, duplicateKey):
					connection, ok := m.selectedItem()
					if !ok {
						return m, nil
					}
					m.duplicate = newDuplicateForm(connection)
					m.status = nil
					m.mode = screenDuplicateForm
					return m, m.duplicate.form.Init()
				case key.Matches(message, deleteKey):
					connection, ok := m.selectedItem()
					if !ok {
						return m, nil
					}
					if m.editor == nil || !m.editor.CanModify(connection) {
						m.status = fmt.Errorf("connection %q cannot be deleted", connection.Name)
						return m, nil
					}
					m.confirm = newDeleteConfirmation(connection)
					m.status = nil
					m.mode = screenDeleteConfirm
					return m, m.confirm.form.Init()
				}
			}
		}
	default:
		switch m.mode {
		case screenConnectionForm:
			return m.updateConnectionForm(msg)
		case screenDuplicateForm:
			return m.updateDuplicateForm(msg)
		case screenDeleteConfirm:
			return m.updateDeleteConfirmation(msg)
		case screenSaving:
			return m, nil
		}
	}

	var command tea.Cmd
	m.list, command = m.list.Update(msg)
	return m, command
}

func (m model) beginConnection() (model, tea.Cmd) {
	connection, ok := m.selectedItem()
	if !ok {
		return m, nil
	}
	m.selected = &connection
	m.action = ActionConnect
	m.quitting = true
	return m, tea.Quit
}

func (m model) selectedItem() (domain.Connection, bool) {
	item, ok := m.list.SelectedItem().(Item)
	if !ok {
		return domain.Connection{}, false
	}
	return item.Connection(), true
}

func (m model) updateConnectionForm(message tea.Msg) (model, tea.Cmd) {
	var command tea.Cmd
	m.form, command = m.form.Update(message)
	return m.submitConnectionForm(command)
}

func (m model) submitConnectionForm(command tea.Cmd) (model, tea.Cmd) {
	if !m.form.Completed() {
		return m, command
	}

	candidate, err := m.form.Candidate()
	if err != nil {
		m.status = err
		m.form = reopenConnectionForm(m.form)
		return m, m.form.form.Init()
	}
	m.status = nil
	m.mode = screenSaving
	original := domain.Connection{}
	if m.form.mode == formEdit {
		original = m.form.original
	}
	return m, tea.Batch(command, m.mutationCommand(commandForForm(m.form), candidate, original, ""))
}

func (m model) updateDuplicateForm(message tea.Msg) (model, tea.Cmd) {
	var command tea.Cmd
	m.duplicate, command = m.duplicate.Update(message)
	if !m.duplicate.Completed() {
		return m, command
	}
	name := m.duplicate.Name()
	if name == "" {
		m.status = fmt.Errorf("name cannot be empty")
		m.duplicate = newDuplicateForm(m.duplicate.source)
		return m, m.duplicate.form.Init()
	}
	m.status = nil
	m.mode = screenSaving
	return m, tea.Batch(command, m.mutationCommand(commandDuplicate, domain.Candidate{}, m.duplicate.source, name))
}

func (m model) updateDeleteConfirmation(message tea.Msg) (model, tea.Cmd) {
	var command tea.Cmd
	m.confirm, command = m.confirm.Update(message)
	if !m.confirm.Completed() {
		return m, command
	}
	if !m.confirm.confirmed {
		m.mode = screenList
		return m, command
	}
	m.status = nil
	m.mode = screenSaving
	return m, tea.Batch(command, m.mutationCommand(commandDeleteConfirm, domain.Candidate{}, m.confirm.source, ""))
}

type mutationMsg struct {
	connections  []domain.Connection
	selectedName string
	selectIndex  int
	err          error
}

func commandForForm(form connectionForm) listCommand {
	if form.mode == formEdit {
		return commandEdit
	}
	return commandAdd
}

func (m model) mutationCommand(command listCommand, candidate domain.Candidate, original domain.Connection, name string) tea.Cmd {
	return func() tea.Msg {
		if m.editor == nil {
			return mutationMsg{err: fmt.Errorf("connection editor is not configured")}
		}
		var (
			connections []domain.Connection
			err         error
		)
		switch command {
		case commandAdd:
			connections, err = m.editor.Create(m.ctx, candidate)
		case commandEdit:
			connections, err = m.editor.Update(m.ctx, original, candidate)
		case commandDuplicate:
			connections, err = m.editor.Duplicate(m.ctx, original, name)
		case commandDeleteConfirm:
			connections, err = m.editor.Delete(m.ctx, original)
		default:
			return mutationMsg{err: fmt.Errorf("unknown picker command")}
		}
		result := mutationMsg{connections: connections, err: err}
		if command == commandDeleteConfirm {
			result.selectIndex = m.list.Index()
		} else {
			result.selectedName = name
			if command == commandAdd || command == commandEdit {
				result.selectedName = strings.TrimSpace(candidate.Name)
			}
		}
		return result
	}
}

func (m model) updateMutation(message mutationMsg) (tea.Model, tea.Cmd) {
	if message.err != nil {
		m.mode = screenList
		m.status = message.err
		return m, nil
	}

	items := make([]list.Item, 0, len(message.connections))
	for _, connection := range message.connections {
		items = append(items, NewItem(connection))
	}
	command := m.list.SetItems(items)
	m.pendingName = ""
	m.pendingSelected = false
	if m.list.IsFiltered() {
		m.pendingSelected = len(items) > 0
		if message.selectedName != "" {
			m.pendingName = message.selectedName
		} else {
			m.pendingIndex = message.selectIndex
		}
	} else if message.selectedName != "" {
		selectItemByName(&m.list, items, message.selectedName)
	} else if len(items) > 0 {
		selectIndex(&m.list, message.selectIndex, len(items))
	}
	m.mode = screenList
	m.status = nil
	return m, command
}

func selectItemByName(component *list.Model, items []list.Item, name string) {
	for index, item := range items {
		connection, ok := item.(Item)
		if ok && connection.Connection().Name == name {
			component.Select(index)
			return
		}
	}
}

func selectIndex(component *list.Model, index, length int) {
	if length == 0 {
		return
	}
	if index >= length {
		index = length - 1
	}
	if index < 0 {
		index = 0
	}
	component.Select(index)
}

func (m *model) applyPendingSelection() {
	if !m.pendingSelected {
		return
	}
	visible := m.list.VisibleItems()
	if m.pendingName != "" {
		selectItemByName(&m.list, visible, m.pendingName)
	} else {
		selectIndex(&m.list, m.pendingIndex, len(visible))
	}
	m.pendingName = ""
	m.pendingSelected = false
}

func reopenConnectionForm(form connectionForm) connectionForm {
	return newConnectionForm(form.mode, form.original, form.values)
}

func (m model) View() tea.View {
	var content string
	switch m.mode {
	case screenConnectionForm:
		content = m.form.View()
	case screenDuplicateForm:
		content = m.duplicate.View()
	case screenDeleteConfirm:
		content = m.confirm.View()
	case screenSaving:
		content = "Saving…"
	default:
		content = m.list.View()
	}
	if m.status != nil {
		content += "\n\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Render(m.status.Error())
	}
	if m.quitting {
		content = ""
	}
	view := tea.NewView(content)
	view.AltScreen = true
	return view
}

func (m model) Selected() (domain.Connection, bool) {
	if m.selected == nil {
		return domain.Connection{}, false
	}
	return *m.selected, true
}

func (m model) Action() Action {
	return m.action
}

func Pick(
	ctx context.Context,
	connections []domain.Connection,
	editor ConnectionEditor,
	input io.Reader,
	output io.Writer,
) (domain.Connection, Action, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	programContext, cancel := context.WithCancel(ctx)
	defer cancel()

	program := tea.NewProgram(
		NewModel(programContext, connections, editor),
		tea.WithContext(programContext),
		tea.WithInput(input),
		tea.WithOutput(output),
	)
	finalModel, err := program.Run()
	if err != nil {
		return domain.Connection{}, ActionNone, err
	}
	result, ok := finalModel.(model)
	if !ok {
		return domain.Connection{}, ActionNone, nil
	}
	connection, selected := result.Selected()
	if !selected {
		return domain.Connection{}, ActionNone, nil
	}
	return connection, result.Action(), nil
}
