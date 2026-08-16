package tui

import (
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/Alurith/hoplane/internal/domain"
	"github.com/Alurith/hoplane/internal/rdpoptions"
	"github.com/Alurith/hoplane/internal/terminal"
)

type formValues struct {
	Name        string
	Protocol    string
	Host        string
	Port        string
	User        string
	Description string
	Tags        string

	RDPClient            string
	RDPDomain            string
	RDPFullscreen        bool
	RDPIgnoreCertificate bool
}

type formMode uint8

const (
	formAdd formMode = iota
	formEdit
)

type connectionForm struct {
	form       *huh.Form
	mode       formMode
	original   domain.Connection
	values     formValues
	liveValues *formValues
}

func newAddForm() connectionForm {
	return newConnectionForm(formAdd, domain.Connection{}, formValues{})
}

func newEditForm(connection domain.Connection) connectionForm {
	values := formValues{
		Name:        connection.Name,
		Protocol:    string(connection.Endpoint.Protocol),
		Host:        connection.Endpoint.Host,
		Port:        strconv.FormatUint(uint64(connection.Endpoint.Port), 10),
		User:        connection.Endpoint.User,
		Description: connection.Description,
		Tags:        formatTags(connection.Tags),
	}
	if options := connection.Options[rdpoptions.Namespace]; options != nil {
		values.RDPClient = options[rdpoptions.Client]
		values.RDPDomain = options[rdpoptions.Domain]
		values.RDPFullscreen, _ = strconv.ParseBool(options[rdpoptions.Fullscreen])
		values.RDPIgnoreCertificate, _ = strconv.ParseBool(options[rdpoptions.IgnoreCertificate])
	}

	return newConnectionForm(formEdit, connection, values)
}

func newConnectionForm(mode formMode, original domain.Connection, values formValues) connectionForm {
	values = sanitizeFormValues(values)
	shared := &values
	name := huh.NewInput().
		Key("name").
		Title("Name").
		Value(&shared.Name).
		Validate(required("name"))
	protocol := huh.NewSelect[string]().
		Key("protocol").
		Title("Protocol").
		Options(protocolOptions()...).
		Value(&shared.Protocol)
	host := huh.NewInput().
		Key("host").
		Title("Host").
		Value(&shared.Host).
		Validate(required("host"))
	port := huh.NewInput().
		Key("port").
		Title("Port").
		Description("Leave empty to use the protocol default.").
		Value(&shared.Port)
	user := huh.NewInput().Key("user").Title("User").Value(&shared.User)
	description := huh.NewInput().Key("description").Title("Description").Value(&shared.Description)
	tags := huh.NewInput().
		Key("tags").
		Title("Tags").
		Description("Comma-separated tags").
		Value(&shared.Tags)

	additionalGroup := huh.NewGroup(user, description, tags).
		Title("Additional data").
		Description("Optional fields · Ctrl+S skips this section")

	rdpGroup := huh.NewGroup(
		huh.NewInput().
			Key("rdp_client").
			Title("Client ID").
			Description("Leave empty for the platform default (Linux: xfreerdp3).").
			Value(&shared.RDPClient),
		huh.NewInput().
			Key("rdp_domain").
			Title("Domain").
			Description("Optional: AD domain or remote computer name for a local account.").
			Value(&shared.RDPDomain),
		huh.NewConfirm().Key("rdp_fullscreen").Title("Fullscreen").Value(&shared.RDPFullscreen),
		huh.NewConfirm().Key("rdp_ignore_certificate").Title("Ignore certificate (INSECURE)").Value(&shared.RDPIgnoreCertificate),
	).Title("RDP options").Description("Optional fields · Ctrl+S skips this section").WithHideFunc(func() bool {
		return !strings.EqualFold(strings.TrimSpace(shared.Protocol), string(domain.ProtocolRDP))
	})

	form := huh.NewForm(
		huh.NewGroup(name, protocol, host, port).Title(formTitle(mode)),
		additionalGroup,
		rdpGroup,
	)
	form.CancelCmd = tea.Quit
	return connectionForm{
		form:       form,
		mode:       mode,
		original:   original,
		values:     values,
		liveValues: shared,
	}
}

func formatTags(tags []string) string {
	formatted := make([]string, 0, len(tags))
	for _, tag := range tags {
		if !strings.ContainsAny(tag, ",\"\r\n") {
			formatted = append(formatted, tag)
			continue
		}

		var encoded strings.Builder
		writer := csv.NewWriter(&encoded)
		_ = writer.Write([]string{tag})
		writer.Flush()
		formatted = append(formatted, strings.TrimSuffix(encoded.String(), "\n"))
	}
	return strings.Join(formatted, ", ")
}

func parseTags(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}

	reader := csv.NewReader(strings.NewReader(value))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	tags, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("parse tags: %w", err)
	}
	return tags, nil
}

func formTitle(mode formMode) string {
	if mode == formEdit {
		return "Edit connection"
	}
	return "Add connection"
}

func protocolOptions() []huh.Option[string] {
	options := []huh.Option[string]{
		huh.NewOption("SSH", string(domain.ProtocolSSH)),
		huh.NewOption("RDP", string(domain.ProtocolRDP)),
	}
	return options
}

func required(field string) func(string) error {
	return func(value string) error {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s cannot be empty", field)
		}
		return nil
	}
}

func (f *connectionForm) syncValues() {
	if f.liveValues != nil {
		f.values = *f.liveValues
	}
}

func (f connectionForm) SkipOptional() (connectionForm, tea.Cmd, bool) {
	if f.form == nil {
		return f, nil, false
	}
	field := f.form.GetFocusedField()
	if field == nil || !optionalField(field.GetKey()) {
		return f, nil, false
	}
	command := f.form.NextGroup()
	f.syncValues()
	return f, command, true
}

func optionalField(field string) bool {
	switch field {
	case "user", "description", "tags", "rdp_client", "rdp_domain", "rdp_fullscreen", "rdp_ignore_certificate":
		return true
	default:
		return false
	}
}

func (f connectionForm) Update(msg tea.Msg) (connectionForm, tea.Cmd) {
	if f.form == nil {
		return f, nil
	}
	if keyPress, ok := msg.(tea.KeyPressMsg); ok && keyPress.String() == "ctrl+v" {
		return f, nil
	}
	if f.liveValues != nil {
		*f.liveValues = f.values
	}
	updated, command := f.form.Update(sanitizeTextMessage(msg))
	if form, ok := updated.(*huh.Form); ok {
		f.form = form
	}
	f.syncValues()
	f.sanitizeValues()
	return f, command
}

func (f connectionForm) Candidate() (domain.Candidate, error) {
	tags, err := parseTags(f.values.Tags)
	if err != nil {
		return domain.Candidate{}, err
	}

	var port *uint16
	if strings.TrimSpace(f.values.Port) != "" {
		value, err := strconv.ParseUint(strings.TrimSpace(f.values.Port), 10, 16)
		if err != nil || value == 0 {
			return domain.Candidate{}, fmt.Errorf("port %q must be between 1 and 65535", f.values.Port)
		}
		parsed := uint16(value)
		port = &parsed
	}

	candidate := domain.Candidate{
		Name:        f.values.Name,
		Protocol:    f.values.Protocol,
		Host:        f.values.Host,
		Port:        port,
		User:        f.values.User,
		Description: f.values.Description,
		Tags:        tags,
		Options:     f.options(),
	}
	if f.mode == formEdit {
		candidate.Source = f.original.Endpoint.Source
	}

	normalized, err := domain.NormalizeCandidate(candidate)
	if err != nil {
		return domain.Candidate{}, err
	}
	switch normalized.Endpoint.Protocol {
	case domain.ProtocolSSH:
		if len(candidate.Options) > 0 {
			return domain.Candidate{}, fmt.Errorf("SSH options are not supported")
		}
	case domain.ProtocolRDP:
		if _, err := rdpoptions.Decode(candidate.Options); err != nil {
			return domain.Candidate{}, err
		}
	}
	return candidate, nil
}

func (f connectionForm) options() domain.Options {
	protocol := strings.ToLower(strings.TrimSpace(f.values.Protocol))
	var options domain.Options
	if f.mode == formEdit {
		options = domain.CloneOptions(f.original.Options)
		originalProtocol := string(f.original.Endpoint.Protocol)
		if protocol != originalProtocol {
			delete(options, originalProtocol)
		}
	}

	switch protocol {
	case string(domain.ProtocolSSH):
		delete(options, rdpoptions.Namespace)
	case string(domain.ProtocolRDP):
		encoded := rdpoptions.Encode(rdpoptions.Options{
			Client:            strings.TrimSpace(f.values.RDPClient),
			Domain:            strings.TrimSpace(f.values.RDPDomain),
			Fullscreen:        f.values.RDPFullscreen,
			IgnoreCertificate: f.values.RDPIgnoreCertificate,
		})
		if len(encoded) > 0 {
			if options == nil {
				options = make(domain.Options)
			}
			for namespace, values := range encoded {
				options[namespace] = values
			}
		} else {
			delete(options, rdpoptions.Namespace)
		}
	default:
		delete(options, rdpoptions.Namespace)
	}
	if len(options) == 0 {
		return nil
	}
	return options
}

func (f connectionForm) Completed() bool {
	return f.form != nil && f.form.State == huh.StateCompleted
}

func (f connectionForm) View() string {
	if f.form == nil {
		return ""
	}
	if f.liveValues != nil {
		*f.liveValues = sanitizeFormValues(f.values)
	}
	return f.form.View()
}

func (f *connectionForm) sanitizeValues() {
	if f.liveValues == nil {
		return
	}
	f.values = sanitizeFormValues(f.values)
	*f.liveValues = f.values
}

func sanitizeTextMessage(message tea.Msg) tea.Msg {
	switch message := message.(type) {
	case tea.KeyPressMsg:
		if message.Text != "" {
			message.Text = terminal.EscapeControls(message.Text)
		}
		return message
	case tea.PasteMsg:
		message.Content = terminal.EscapeControls(message.Content)
		return message
	default:
		return message
	}
}

func sanitizeFormValues(values formValues) formValues {
	values.Name = terminal.EscapeControls(values.Name)
	values.Protocol = terminal.EscapeControls(values.Protocol)
	values.Host = terminal.EscapeControls(values.Host)
	values.Port = terminal.EscapeControls(values.Port)
	values.User = terminal.EscapeControls(values.User)
	values.Description = terminal.EscapeControls(values.Description)
	values.Tags = terminal.EscapeControls(values.Tags)
	values.RDPClient = terminal.EscapeControls(values.RDPClient)
	values.RDPDomain = terminal.EscapeControls(values.RDPDomain)
	return values
}

type duplicateForm struct {
	form   *huh.Form
	source domain.Connection
	name   string
}

func newDuplicateForm(source domain.Connection) duplicateForm {
	name := terminal.EscapeControls(source.Name + "-copy")
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Key("name").Title("Name").Value(&name).Validate(required("name")),
		).Title("Duplicate connection"),
	)
	form.CancelCmd = tea.Quit
	return duplicateForm{form: form, source: source, name: name}
}

func (f duplicateForm) Update(msg tea.Msg) (duplicateForm, tea.Cmd) {
	if f.form == nil {
		return f, nil
	}
	if keyPress, ok := msg.(tea.KeyPressMsg); ok && keyPress.String() == "ctrl+v" {
		return f, nil
	}
	updated, command := f.form.Update(sanitizeTextMessage(msg))
	if form, ok := updated.(*huh.Form); ok {
		f.form = form
	}
	if value, ok := f.form.Get("name").(string); ok {
		f.name = value
	}
	return f, command
}

func (f duplicateForm) Completed() bool {
	return f.form != nil && f.form.State == huh.StateCompleted
}

func (f duplicateForm) Name() string {
	return strings.TrimSpace(f.name)
}

func (f duplicateForm) View() string {
	if f.form == nil {
		return ""
	}
	return f.form.View()
}

type deleteConfirmation struct {
	form      *huh.Form
	source    domain.Connection
	confirmed bool
}

func newDeleteConfirmation(source domain.Connection) deleteConfirmation {
	confirmed := false
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewConfirm().
				Key("confirmed").
				Title("Delete connection?").
				Affirmative("Delete").
				Negative("Cancel").
				Value(&confirmed),
		),
	)
	form.CancelCmd = tea.Quit
	return deleteConfirmation{form: form, source: source, confirmed: confirmed}
}

func (f deleteConfirmation) Update(msg tea.Msg) (deleteConfirmation, tea.Cmd) {
	if f.form == nil {
		return f, nil
	}
	updated, command := f.form.Update(msg)
	if form, ok := updated.(*huh.Form); ok {
		f.form = form
	}
	if value, ok := f.form.Get("confirmed").(bool); ok {
		f.confirmed = value
	}
	return f, command
}

func (f deleteConfirmation) Completed() bool {
	return f.form != nil && f.form.State == huh.StateCompleted
}

func (f deleteConfirmation) View() string {
	if f.form == nil {
		return ""
	}
	return f.form.View()
}
