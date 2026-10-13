package cli

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Alurith/hoplane/internal/config"
	"github.com/Alurith/hoplane/internal/domain"
	"github.com/Alurith/hoplane/internal/secrets"
)

func testPickerEditor(t *testing.T) (*pickerEditor, domain.Connection) {
	t.Helper()
	state := &commandState{configPath: filepath.Join(t.TempDir(), "config.yaml"), secretStore: secrets.NewMemoryStore()}
	editor := newPickerEditor(state)
	connections, err := editor.Create(context.Background(), domain.Candidate{Name: "nas", Protocol: "ssh", Host: "nas.local", User: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	return editor, connections[0]
}

func testCandidate(connection domain.Connection) domain.Candidate {
	return config.EntryFromConnection(connection).Candidate(connection.Endpoint.Source)
}

func TestPickerEditorRejectsStaleUpdate(t *testing.T) {
	editor, original := testPickerEditor(t)
	other := newPickerEditor(editor.state)
	first := testCandidate(original)
	first.Host = "new.local"
	if _, err := other.Update(context.Background(), original, first); err != nil {
		t.Fatal(err)
	}
	second := testCandidate(original)
	second.Description = "new description"
	if _, err := editor.Update(context.Background(), original, second); err == nil || !strings.Contains(err.Error(), "changed") {
		t.Errorf("stale update error = %v", err)
	}
	connections, err := editor.reload(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if connections[0].Endpoint.Host != "new.local" || connections[0].Description != "" {
		t.Fatal("stale form overwrote the saved connection")
	}
}

func TestPickerEditorStaleProtocolDoesNotClearSecrets(t *testing.T) {
	editor, original := testPickerEditor(t)
	candidate := testCandidate(original)
	candidate.Protocol = "rdp"
	connections, err := editor.Update(context.Background(), original, candidate)
	if err != nil {
		t.Fatal(err)
	}
	current := connections[0]
	if err := editor.SetPassword(context.Background(), current, []byte("rdp-password")); err != nil {
		t.Fatal(err)
	}
	if _, err := editor.Update(context.Background(), original, testCandidate(original)); err == nil {
		t.Error("stale protocol update succeeded")
	}
	got, err := editor.state.secretStore.Lookup(context.Background(), passwordRef(current))
	defer secrets.Clear(got)
	if err != nil || string(got) != "rdp-password" {
		t.Fatalf("current password changed: %v", err)
	}
	connections, err = editor.reload(context.Background())
	if err != nil || connections[0].Endpoint.Protocol != domain.ProtocolRDP {
		t.Fatalf("current protocol changed: %v", err)
	}
}

func TestStorePasswordRejectsChangedTarget(t *testing.T) {
	for _, field := range []string{"protocol", "host", "user", "port", "options", "name"} {
		t.Run(field, func(t *testing.T) {
			editor, original := testPickerEditor(t)
			candidate := testCandidate(original)
			switch field {
			case "protocol":
				candidate.Protocol = "rdp"
			case "host":
				candidate.Host = "other.local"
			case "user":
				candidate.User = "bob"
			case "port":
				port := uint16(2222)
				candidate.Port = &port
			case "options":
				// Use RDP so a changed authentication domain is valid config.
				candidate.Protocol = "rdp"
				connections, err := editor.Update(context.Background(), original, candidate)
				if err != nil {
					t.Fatal(err)
				}
				original = connections[0]
				candidate = testCandidate(original)
				candidate.Options = domain.Options{"rdp": {"domain": "OTHER"}}
			case "name":
				candidate.Name = "renamed"
			}
			connections, err := editor.Update(context.Background(), original, candidate)
			if err != nil {
				t.Fatal(err)
			}
			current := connections[0]
			err = editor.SetPassword(context.Background(), original, []byte("new-password"))
			if field == "name" {
				if err != nil {
					t.Fatalf("rename broke password storage: %v", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "changed") {
				t.Errorf("changed target error = %v", err)
			}
			got, lookupErr := editor.state.secretStore.Lookup(context.Background(), passwordRef(current))
			defer secrets.Clear(got)
			if field == "name" {
				if lookupErr != nil || string(got) != "new-password" {
					t.Fatalf("renamed password not stored: %v", lookupErr)
				}
			} else if !errors.Is(lookupErr, secrets.ErrNotFound) {
				t.Errorf("password written to changed target: %v", lookupErr)
			}
		})
	}
}
