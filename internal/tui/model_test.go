package tui

import (
	"context"
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Alurith/hoplane/internal/domain"
	"github.com/Alurith/hoplane/internal/sshoptions"
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

func TestEditFormPreservesSSHConfigReference(t *testing.T) {
	connection := domain.Connection{
		Name: "nas-copy",
		Endpoint: domain.Endpoint{
			Protocol: domain.ProtocolSSH,
			Host:     "nas",
			Port:     22,
		},
		Options: domain.Options{
			sshoptions.Namespace: {
				sshoptions.ConfigFile:   "/home/alice/.ssh/config",
				sshoptions.HostAlias:    "nas",
				sshoptions.IdentityFile: "~/.ssh/id_ed25519",
			},
		},
	}

	form := newEditForm(connection)
	options := form.options()
	ssh := options[sshoptions.Namespace]
	if ssh[sshoptions.ConfigFile] != "/home/alice/.ssh/config" || ssh[sshoptions.HostAlias] != "nas" {
		t.Fatalf("SSH config reference = %#v, want preserved reference", ssh)
	}
	if ssh[sshoptions.IdentityFile] != "~/.ssh/id_ed25519" {
		t.Fatalf("SSH identity file = %#v, want preserved form value", ssh)
	}
}

func TestEditFormCanCorrectInvalidStoredOptions(t *testing.T) {
	connection := domain.Connection{
		Name: "nas",
		Endpoint: domain.Endpoint{
			Protocol: domain.ProtocolSSH,
			Host:     "nas.local",
			Port:     22,
		},
		Options: domain.Options{sshoptions.Namespace: {"unknown": "value"}},
	}

	if _, err := newEditForm(connection).Candidate(); err != nil {
		t.Fatalf("Candidate() error = %v, want form to rebuild options", err)
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
