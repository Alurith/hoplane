package tui

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/Alurith/hoplane/internal/domain"
	"github.com/Alurith/hoplane/internal/rdpoptions"
	"github.com/Alurith/hoplane/internal/sshoptions"
)

type formValues struct {
	Name        string
	Protocol    string
	Host        string
	Port        string
	User        string
	Description string
	Tags        string

	SSHIdentityFile string
	SSHProxyJump    string

	RDPClient            string
	RDPFullscreen        bool
	RDPIgnoreCertificate bool
}

type formMode uint8

const (
	formAdd formMode = iota
	formEdit
)

type connectionForm struct {
	form        *huh.Form
	mode        formMode
	original    domain.Connection
	values      formValues
	liveValues  *formValues
	optionError error
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
		Tags:        strings.Join(connection.Tags, ", "),
	}

	if options := connection.Options[sshoptions.Namespace]; options != nil {
		values.SSHIdentityFile = options[sshoptions.IdentityFile]
		values.SSHProxyJump = options[sshoptions.ProxyJump]
	}
	if options := connection.Options[rdpoptions.Namespace]; options != nil {
		values.RDPClient = options[rdpoptions.Client]
		values.RDPFullscreen, _ = strconv.ParseBool(options[rdpoptions.Fullscreen])
		values.RDPIgnoreCertificate, _ = strconv.ParseBool(options[rdpoptions.IgnoreCertificate])
	}

	form := newConnectionForm(formEdit, connection, values)
	switch connection.Endpoint.Protocol {
	case domain.ProtocolSSH:
		_, form.optionError = sshoptions.Decode(connection.Options)
	case domain.ProtocolRDP:
		_, form.optionError = rdpoptions.Decode(connection.Options)
	}
	return form
}

func newConnectionForm(mode formMode, original domain.Connection, values formValues) connectionForm {
	shared := &values
	name := huh.NewInput().
		Key("name").
		Title("Name").
		Value(&shared.Name).
		Validate(required("name"))
	protocol := huh.NewSelect[string]().
		Key("protocol").
		Title("Protocol").
		Options(protocolOptions(shared.Protocol)...).
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

	sshGroup := huh.NewGroup(
		huh.NewInput().Key("ssh_identity_file").Title("Identity file").Value(&shared.SSHIdentityFile),
		huh.NewInput().Key("ssh_proxy_jump").Title("Proxy jump").Value(&shared.SSHProxyJump),
	).Title("SSH options").Description("Optional fields · Ctrl+S skips this section").WithHideFunc(func() bool {
		return !strings.EqualFold(strings.TrimSpace(shared.Protocol), string(domain.ProtocolSSH))
	})

	rdpGroup := huh.NewGroup(
		huh.NewInput().Key("rdp_client").Title("Client").Value(&shared.RDPClient),
		huh.NewConfirm().Key("rdp_fullscreen").Title("Fullscreen").Value(&shared.RDPFullscreen),
		huh.NewConfirm().Key("rdp_ignore_certificate").Title("Ignore certificate").Value(&shared.RDPIgnoreCertificate),
	).Title("RDP options").Description("Optional fields · Ctrl+S skips this section").WithHideFunc(func() bool {
		return !strings.EqualFold(strings.TrimSpace(shared.Protocol), string(domain.ProtocolRDP))
	})

	form := huh.NewForm(
		huh.NewGroup(name, protocol, host, port).Title(formTitle(mode)),
		additionalGroup,
		sshGroup,
		rdpGroup,
	)
	return connectionForm{
		form:       form,
		mode:       mode,
		original:   original,
		values:     values,
		liveValues: shared,
	}
}

func formTitle(mode formMode) string {
	if mode == formEdit {
		return "Edit connection"
	}
	return "Add connection"
}

func protocolOptions(current string) []huh.Option[string] {
	options := []huh.Option[string]{
		huh.NewOption("SSH", string(domain.ProtocolSSH)),
		huh.NewOption("RDP", string(domain.ProtocolRDP)),
	}
	current = strings.ToLower(strings.TrimSpace(current))
	if current != "" && current != string(domain.ProtocolSSH) && current != string(domain.ProtocolRDP) {
		options = append(options, huh.NewOption("Current ("+current+")", current))
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
	case "user", "description", "tags", "ssh_identity_file", "ssh_proxy_jump", "rdp_client", "rdp_fullscreen", "rdp_ignore_certificate":
		return true
	default:
		return false
	}
}

func (f connectionForm) Update(msg tea.Msg) (connectionForm, tea.Cmd) {
	if f.form == nil {
		return f, nil
	}
	if f.liveValues != nil {
		*f.liveValues = f.values
	}
	updated, command := f.form.Update(msg)
	if form, ok := updated.(*huh.Form); ok {
		f.form = form
	}
	f.syncValues()
	return f, command
}

func (f connectionForm) Candidate() (domain.Candidate, error) {
	if f.optionError != nil {
		return domain.Candidate{}, f.optionError
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
		Tags:        strings.Split(f.values.Tags, ","),
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
		if _, err := sshoptions.Decode(candidate.Options); err != nil {
			return domain.Candidate{}, err
		}
	case domain.ProtocolRDP:
		if _, err := rdpoptions.Decode(candidate.Options); err != nil {
			return domain.Candidate{}, err
		}
	}
	return candidate, nil
}

func (f connectionForm) options() domain.Options {
	var options domain.Options
	if f.mode == formEdit {
		options = domain.CloneOptions(f.original.Options)
		delete(options, string(f.original.Endpoint.Protocol))
	}

	protocol := strings.ToLower(strings.TrimSpace(f.values.Protocol))
	switch protocol {
	case string(domain.ProtocolSSH):
		delete(options, rdpoptions.Namespace)
		values := make(map[string]string, 2)
		if strings.TrimSpace(f.values.SSHIdentityFile) != "" {
			values[sshoptions.IdentityFile] = strings.TrimSpace(f.values.SSHIdentityFile)
		}
		if strings.TrimSpace(f.values.SSHProxyJump) != "" {
			values[sshoptions.ProxyJump] = strings.TrimSpace(f.values.SSHProxyJump)
		}
		if len(values) > 0 {
			if options == nil {
				options = make(domain.Options)
			}
			options[sshoptions.Namespace] = values
		}
	case string(domain.ProtocolRDP):
		delete(options, sshoptions.Namespace)
		encoded := rdpoptions.Encode(rdpoptions.Options{
			Client:            strings.TrimSpace(f.values.RDPClient),
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
		}
	default:
		delete(options, sshoptions.Namespace)
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
		*f.liveValues = f.values
	}
	return f.form.View()
}

type duplicateForm struct {
	form   *huh.Form
	source domain.Connection
	name   string
}

func newDuplicateForm(source domain.Connection) duplicateForm {
	name := source.Name + "-copy"
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Key("name").Title("Name").Value(&name).Validate(required("name")),
		).Title("Duplicate connection"),
	)
	return duplicateForm{form: form, source: source, name: name}
}

func (f duplicateForm) Update(msg tea.Msg) (duplicateForm, tea.Cmd) {
	if f.form == nil {
		return f, nil
	}
	updated, command := f.form.Update(msg)
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
