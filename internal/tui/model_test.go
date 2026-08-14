package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/Alurith/hoplane/internal/domain"
)

func TestModelSelectsConnection(t *testing.T) {
	initial := NewModel([]domain.Connection{{
		Name: "nas",
		Endpoint: domain.Endpoint{
			Protocol: domain.ProtocolSSH,
			Host:     "nas.local",
			Port:     22,
		},
	}})

	updated, command := initial.Update(tea.KeyPressMsg(tea.Key{Text: "enter"}))
	if command == nil {
		t.Fatal("Update() command = nil, want quit command")
	}
	result := updated.(model)
	selected, ok := result.Selected()
	if !ok || selected.Name != "nas" {
		t.Fatalf("selected = %#v, ok = %v", selected, ok)
	}
	if result.Action() != ActionSelect {
		t.Fatalf("action = %v, want %v", result.Action(), ActionSelect)
	}
}

func TestModelConnectsConnection(t *testing.T) {
	initial := NewModel([]domain.Connection{{
		Name: "nas",
		Endpoint: domain.Endpoint{
			Protocol: domain.ProtocolSSH,
			Host:     "nas.local",
			Port:     22,
		},
	}})

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
