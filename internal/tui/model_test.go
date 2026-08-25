package tui

import (
	"context"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Alurith/hoplane/internal/domain"
	"github.com/Alurith/hoplane/internal/secrets"
)

func TestModelSelectsConnection(t *testing.T) {
	initial := NewModel(context.Background(), []domain.Connection{{
		Name: "nas",
		Endpoint: domain.Endpoint{
			Protocol: domain.ProtocolSSH,
			Host:     "nas.local",
			Port:     22,
		},
	}}, nil)

	updated, command := initial.Update(tea.KeyPressMsg(tea.Key{Text: "enter"}))
	if command == nil {
		t.Fatal("Update() command = nil, want quit command")
	}
	result := updated.(model)
	selected, ok := result.Selected()
	if !ok || selected.Name != "nas" {
		t.Fatalf("selected = %#v, ok = %v", selected, ok)
	}
	if result.Action() != ActionConnect {
		t.Fatalf("action = %v, want %v", result.Action(), ActionConnect)
	}
}

func TestModelConnectsConnection(t *testing.T) {
	initial := NewModel(context.Background(), []domain.Connection{{
		Name: "nas",
		Endpoint: domain.Endpoint{
			Protocol: domain.ProtocolSSH,
			Host:     "nas.local",
			Port:     22,
		},
	}}, nil)

	updated, command := initial.Update(tea.KeyPressMsg(tea.Key{Text: "c"}))
	if command == nil {
		t.Fatal("Update() command = nil, want quit command")
	}
	result := updated.(model)
	selected, ok := result.Selected()
	if !ok || selected.Name != "nas" {
		t.Fatalf("selected = %#v, ok = %v", selected, ok)
	}
	if result.Action() != ActionConnect {
		t.Fatalf("action = %v, want %v", result.Action(), ActionConnect)
	}
}

func TestEditFormPreservesCommaInTags(t *testing.T) {
	connection := domain.Connection{
		Name: "nas",
		Endpoint: domain.Endpoint{
			Protocol: domain.ProtocolSSH,
			Host:     "nas.local",
			Port:     22,
		},
		Tags: []string{"work", "rack,2"},
	}

	candidate, err := newEditForm(connection).Candidate()
	if err != nil {
		t.Fatalf("Candidate() error = %v", err)
	}
	if !reflect.DeepEqual(candidate.Tags, connection.Tags) {
		t.Fatalf("tags = %#v, want %#v", candidate.Tags, connection.Tags)
	}
}

func TestConnectionFormKeepsPasswordOutOfCandidate(t *testing.T) {
	form := newAddForm()
	form.values = formValues{
		Name:                 "office",
		Protocol:             "rdp",
		Host:                 "desktop.example.com",
		PasswordAction:       passwordActionSet,
		Password:             "super-secret",
		PasswordConfirmation: "super-secret",
	}
	candidate, err := form.Candidate()
	if err != nil {
		t.Fatalf("Candidate() error = %v", err)
	}
	if len(candidate.Options) != 0 {
		t.Fatalf("candidate options = %#v, want no password data", candidate.Options)
	}
	change, err := form.SecretChange()
	if err != nil {
		t.Fatalf("SecretChange() error = %v", err)
	}
	if !change.set || string(change.value) != "super-secret" {
		t.Fatalf("secret change = %#v", change)
	}
	secrets.Clear(change.value)
}

func TestNewAndEditFormsUseDifferentPasswordDefaults(t *testing.T) {
	if got := newAddForm().values.PasswordAction; got != passwordActionSet {
		t.Fatalf("new form password action = %q, want %q", got, passwordActionSet)
	}
	if got := newEditForm(domain.Connection{}).values.PasswordAction; got != passwordActionKeep {
		t.Fatalf("edit form password action = %q, want %q", got, passwordActionKeep)
	}
}

func TestSecretChangeRejectsEmptyPasswordWhenSet(t *testing.T) {
	form := newAddForm()
	form.values.Protocol = "ssh"
	form.values.PasswordAction = passwordActionSet

	_, err := form.SecretChange()
	if err == nil || err.Error() != "password cannot be empty" {
		t.Fatalf("SecretChange() error = %v, want empty password error", err)
	}
}

func TestSecretChangeRequiresMatchingConfirmation(t *testing.T) {
	form := newAddForm()
	form.values.Protocol = "ssh"
	form.values.PasswordAction = passwordActionSet
	form.values.Password = "super-secret"
	form.values.PasswordConfirmation = "different"

	_, err := form.SecretChange()
	if err == nil || err.Error() != "passwords do not match" {
		t.Fatalf("SecretChange() error = %v, want confirmation error", err)
	}
}

func TestSecretChangeRemovesPasswordExplicitly(t *testing.T) {
	form := newEditForm(domain.Connection{})
	form.values.PasswordAction = passwordActionRemove

	change, err := form.SecretChange()
	if err != nil {
		t.Fatalf("SecretChange() error = %v", err)
	}
	if !change.clear || change.set || change.value != nil {
		t.Fatalf("secret change = %#v, want clear only", change)
	}
}

func TestSecretChangeKeepsPasswordByDefault(t *testing.T) {
	form := newEditForm(domain.Connection{})
	form.values.Password = "must-not-be-used"
	form.values.PasswordConfirmation = "must-not-be-used"

	change, err := form.SecretChange()
	if err != nil {
		t.Fatalf("SecretChange() error = %v", err)
	}
	if change.set || change.clear || change.value != nil {
		t.Fatalf("secret change = %#v, want no change", change)
	}
}

func TestOptionalFormHelpIncludesSkipAction(t *testing.T) {
	form := newAddForm()
	form.form.Init()
	form.form.NextGroup()

	bindings := form.form.KeyBinds()
	if len(bindings) == 0 || bindings[0].Help().Key != "ctrl+s" || bindings[0].Help().Desc != "skip" {
		t.Fatalf("optional form help = %#v, want Ctrl+S skip action first", bindings)
	}
}

func TestOptionalFormViewIncludesSkipHelp(t *testing.T) {
	form := newAddForm()
	form.form.Init()
	form.form.NextGroup()
	form.form.Update(tea.WindowSizeMsg{Width: 40, Height: 30})

	view := form.form.View()
	if !strings.Contains(view, "ctrl+s") {
		t.Fatalf("optional form view = %q, want Ctrl+S help", view)
	}
}

func TestRDPFormLoadsDomainWhenEditing(t *testing.T) {
	connection := domain.Connection{
		Name: "office",
		Endpoint: domain.Endpoint{
			Protocol: domain.ProtocolRDP,
			Host:     "desktop.example.com",
			Port:     3389,
		},
		Options: domain.Options{"rdp": {"domain": "CONTOSO"}},
	}
	form := newEditForm(connection)
	candidate, err := form.Candidate()
	if err != nil {
		t.Fatalf("Candidate() error = %v", err)
	}
	if got := candidate.Options["rdp"]["domain"]; got != "CONTOSO" {
		t.Fatalf("domain = %q, want CONTOSO", got)
	}
}

func TestModelAppliesMutationSelectionAfterFiltering(t *testing.T) {
	initial := NewModel(context.Background(), []domain.Connection{{
		Name:     "nas",
		Endpoint: domain.Endpoint{Protocol: domain.ProtocolSSH, Host: "nas.local", Port: 22},
	}}, nil)
	initial.list.SetFilterText("nas")

	updated, command := initial.updateMutation(mutationMsg{
		connections: []domain.Connection{
			{Name: "other", Endpoint: domain.Endpoint{Protocol: domain.ProtocolSSH, Host: "other.local", Port: 22}},
			{Name: "nas-new", Endpoint: domain.Endpoint{Protocol: domain.ProtocolSSH, Host: "nas.local", Port: 22}},
		},
		selectedName: "nas-new",
	})
	if command == nil {
		t.Fatal("updateMutation() command = nil, want filter refresh command")
	}

	filtered, ok := command().(tea.Msg)
	if !ok {
		t.Fatal("filter command returned nil message")
	}
	updated, _ = updated.(model).Update(filtered)
	result := updated.(model)
	selected, ok := result.list.SelectedItem().(Item)
	if !ok || selected.Connection().Name != "nas-new" {
		t.Fatalf("selected item = %#v, want nas-new", result.list.SelectedItem())
	}
}
