package tui

import (
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"charm.land/lipgloss/v2"

	"github.com/Alurith/hoplane/internal/domain"
	"github.com/Alurith/hoplane/internal/rdpoptions"
	"github.com/Alurith/hoplane/internal/secrets"
	"github.com/Alurith/hoplane/internal/terminal"
)

type formValues struct {
	Name                 string
	Protocol             string
	Host                 string
	Port                 string
	User                 string
	Description          string
	Tags                 string
	Password             string
	PasswordAction       string
	PasswordConfirmation string

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

const (
	passwordActionSet    = "set"
	passwordActionNone   = "none"
	passwordActionKeep   = "keep"
	passwordActionRemove = "remove"
)

type connectionForm struct {
	form                      *huh.Form
	mode                      formMode
	original                  domain.Connection
	values                    formValues
	liveValues                *formValues
	passwordInput             *huh.Input
	passwordConfirmationInput *huh.Input
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
	if values.PasswordAction == "" {
		values.PasswordAction = defaultPasswordAction(mode)
	}
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

	additionalGroup := huh.NewGroup(
		withSkipHelp(user),
		withSkipHelp(description),
		withSkipHelp(tags),
	).
		Title("Additional data").
		Description("Optional fields")

	passwordAction := huh.NewSelect[string]().
		Key("password_action").
		Title("Password").
		Description("Choose how to handle the stored password.").
		Options(passwordActionOptions(mode)...).
		Value(&shared.PasswordAction)
	passwordInput := huh.NewInput().
		Key("password").
		Title("New password").
		Description("Enter the new password.").
		EchoMode(huh.EchoModePassword).
		// CharLimit is only a UI guard; every submit path validates by protocol.
		CharLimit(secrets.MaxRDPPasswordBytes).
		Value(&shared.Password)
	passwordConfirmationInput := huh.NewInput().
		Key("password_confirmation").
		Title("Confirm password").
		Description("Enter the new password again.").
		EchoMode(huh.EchoModePassword).
		// CharLimit is only a UI guard; every submit path validates by protocol.
		CharLimit(secrets.MaxRDPPasswordBytes).
		Value(&shared.PasswordConfirmation)
	secretActionGroup := huh.NewGroup(
		withSkipHelp(passwordAction),
	).Title("Credentials")
	secretInputGroup := huh.NewGroup(
		withSkipHelp(passwordInput),
		withSkipHelp(passwordConfirmationInput),
	).Title("Set password").WithHideFunc(func() bool {
		return shared.PasswordAction != passwordActionSet
	})

	rdpGroup := huh.NewGroup(
		withSkipHelp(huh.NewInput().
			Key("rdp_client").
			Title("Client ID").
			Description("Leave empty for the platform default (Linux: xfreerdp3).").
			Value(&shared.RDPClient)),
		withSkipHelp(huh.NewInput().
			Key("rdp_domain").
			Title("Domain").
			Description("Optional: AD domain or remote computer name for a local account.").
			Value(&shared.RDPDomain)),
		withSkipHelp(huh.NewConfirm().
			Key("rdp_fullscreen").
			Title("Fullscreen").
			WithButtonAlignment(lipgloss.Left).
			Value(&shared.RDPFullscreen)),
		withSkipHelp(huh.NewConfirm().
			Key("rdp_ignore_certificate").
			Title("Ignore certificate (INSECURE)").
			WithButtonAlignment(lipgloss.Left).
			Value(&shared.RDPIgnoreCertificate)),
	).Title("RDP options").Description("Optional fields").WithHideFunc(func() bool {
		return !strings.EqualFold(strings.TrimSpace(shared.Protocol), string(domain.ProtocolRDP))
	})

	form := huh.NewForm(
		huh.NewGroup(name, protocol, host, port).Title(formTitle(mode)),
		additionalGroup,
		secretActionGroup,
		secretInputGroup,
		rdpGroup,
	)
	form.CancelCmd = func() tea.Msg {
		shared.Password = ""
		shared.PasswordConfirmation = ""
		passwordInput.Value(&shared.Password)
		passwordConfirmationInput.Value(&shared.PasswordConfirmation)
		return tea.Quit()
	}
	return connectionForm{
		form:                      form,
		mode:                      mode,
		original:                  original,
		values:                    values,
		liveValues:                shared,
		passwordInput:             passwordInput,
		passwordConfirmationInput: passwordConfirmationInput,
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

func defaultPasswordAction(mode formMode) string {
	if mode == formEdit {
		return passwordActionKeep
	}
	return passwordActionSet
}

func passwordActionOptions(mode formMode) []huh.Option[string] {
	if mode == formEdit {
		return []huh.Option[string]{
			huh.NewOption("Keep current password", passwordActionKeep),
			huh.NewOption("Set a new password", passwordActionSet),
			huh.NewOption("Remove stored password", passwordActionRemove),
		}
	}
	return []huh.Option[string]{
		huh.NewOption("Set password", passwordActionSet),
		huh.NewOption("No password", passwordActionNone),
	}
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

type skipHelpField struct {
	huh.Field
}

func (f skipHelpField) GetValue() any {
	if f.Field.GetKey() == "password" || f.Field.GetKey() == "password_confirmation" {
		return ""
	}
	return f.Field.GetValue()
}

func (f skipHelpField) Update(msg tea.Msg) (huh.Model, tea.Cmd) {
	model, command := f.Field.Update(msg)
	field, ok := model.(huh.Field)
	if !ok {
		return model, command
	}
	return skipHelpField{Field: field}, command
}

func (f skipHelpField) KeyBinds() []key.Binding {
	bindings := append([]key.Binding{skipKey}, f.Field.KeyBinds()...)
	return bindings
}

func withSkipHelp(field huh.Field) huh.Field {
	return skipHelpField{Field: field}
}

func optionalField(field string) bool {
	switch field {
	case "user", "description", "tags", "password_action", "password", "password_confirmation", "rdp_client", "rdp_domain", "rdp_fullscreen", "rdp_ignore_certificate":
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
	formMessage := sanitizeTextMessage(msg)
	if field := f.form.GetFocusedField(); field != nil && (field.GetKey() == "password" || field.GetKey() == "password_confirmation") {
		formMessage = msg
	}
	updated, command := f.form.Update(formMessage)
	if form, ok := updated.(*huh.Form); ok {
		f.form = form
	}
	f.syncValues()
	f.sanitizeValues()
	if f.values.PasswordAction != passwordActionSet {
		f.clearPassword()
	}
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
		candidate.ID = f.original.ID
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

type secretChange struct {
	value []byte
	set   bool
	clear bool
}

func (f connectionForm) SecretChange() (secretChange, error) {
	action := f.values.PasswordAction
	if action == "" {
		action = defaultPasswordAction(f.mode)
	}
	switch action {
	case passwordActionKeep, passwordActionNone:
		return secretChange{}, nil
	case passwordActionRemove:
		if f.mode != formEdit {
			return secretChange{}, fmt.Errorf("remove password is only available when editing")
		}
		return secretChange{clear: true}, nil
	case passwordActionSet:
		if f.values.Password == "" {
			return secretChange{}, fmt.Errorf("password cannot be empty")
		}
		if f.values.Password != f.values.PasswordConfirmation {
			return secretChange{}, fmt.Errorf("passwords do not match")
		}
		value := []byte(f.values.Password)
		if err := secrets.ValidateSecretForProtocol(strings.ToLower(strings.TrimSpace(f.values.Protocol)), value); err != nil {
			secrets.Clear(value)
			return secretChange{}, err
		}
		return secretChange{value: value, set: true}, nil
	default:
		return secretChange{}, fmt.Errorf("unknown password action %q", action)
	}
}

func (f *connectionForm) clearPassword() {
	f.values.Password = ""
	f.values.PasswordConfirmation = ""
	if f.liveValues != nil {
		f.liveValues.Password = ""
		f.liveValues.PasswordConfirmation = ""
		if f.passwordInput != nil {
			f.passwordInput.Value(&f.liveValues.Password)
		}
		if f.passwordConfirmationInput != nil {
			f.passwordConfirmationInput.Value(&f.liveValues.PasswordConfirmation)
		}
	}
}

func (f *connectionForm) dispose() {
	f.clearPassword()
	f.form = nil
	f.liveValues = nil
	f.passwordInput = nil
	f.passwordConfirmationInput = nil
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
