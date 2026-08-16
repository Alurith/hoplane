package tui

import (
	"context"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Alurith/hoplane/internal/domain"
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

func TestRDPFormUsesLogicalXFREERDP3ClientID(t *testing.T) {
	form := newAddForm()
	form.values = formValues{
		Name:                 "office",
		Protocol:             "rdp",
		Host:                 "desktop.example.com",
		RDPClient:            "xfreerdp3",
		RDPDomain:            "CONTOSO",
		RDPFullscreen:        true,
		RDPIgnoreCertificate: true,
	}

	candidate, err := form.Candidate()
	if err != nil {
		t.Fatalf("Candidate() error = %v", err)
	}
	want := domain.Options{"rdp": {
		"client":             "xfreerdp3",
		"domain":             "CONTOSO",
		"fullscreen":         "true",
		"ignore_certificate": "true",
	}}
	if !reflect.DeepEqual(candidate.Options, want) {
		t.Fatalf("options = %#v, want %#v", candidate.Options, want)
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

func TestRDPFormRejectsExecutablePathAsClientID(t *testing.T) {
	form := newAddForm()
	form.values = formValues{
		Name:      "office",
		Protocol:  "rdp",
		Host:      "desktop.example.com",
		RDPClient: "/usr/bin/xfreerdp3",
	}

	_, err := form.Candidate()
	if err == nil || !strings.Contains(err.Error(), "must be a logical client ID") {
		t.Fatalf("Candidate() error = %v, want logical client ID rejection", err)
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
