package cli

import (
	"context"
	"errors"
	"os"
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

type secretStoreBackend interface{ secrets.Store }

type failingSecretStore struct {
	secretStoreBackend
	delete func(context.Context, secrets.Ref) error
	store  func(context.Context, secrets.Ref, []byte) error
}

func (s failingSecretStore) Delete(ctx context.Context, ref secrets.Ref) error {
	if s.delete != nil {
		return s.delete(ctx, ref)
	}
	return s.secretStoreBackend.Delete(ctx, ref)
}

func (s failingSecretStore) Store(ctx context.Context, ref secrets.Ref, value []byte) error {
	if s.store != nil {
		return s.store(ctx, ref, value)
	}
	return s.secretStoreBackend.Store(ctx, ref, value)
}

func TestPickerEditorFailedSavePreservesPassword(t *testing.T) {
	for _, operation := range []string{"delete", "protocol"} {
		t.Run(operation, func(t *testing.T) {
			editor, original := testPickerEditor(t)
			store := editor.state.secretStore
			if err := store.Store(context.Background(), passwordRef(original), []byte("original-password")); err != nil {
				t.Fatal(err)
			}
			path := editor.state.configPath
			backup := path + ".backup"
			editor.state.secretStore = failingSecretStore{secretStoreBackend: store, delete: func(ctx context.Context, ref secrets.Ref) error {
				if err := store.Delete(ctx, ref); err != nil {
					return err
				}
				if ref.Protocol == secrets.ProtocolSSH {
					if err := os.Rename(path, backup); err != nil {
						return err
					}
					return os.Mkdir(path, 0o700)
				}
				return nil
			}}
			var err error
			if operation == "delete" {
				_, err = editor.Delete(context.Background(), original)
			} else {
				candidate := testCandidate(original)
				candidate.Protocol = "rdp"
				_, err = editor.Update(context.Background(), original, candidate)
			}
			if err == nil {
				t.Fatal("mutation succeeded despite config save failure")
			}
			assertPassword(t, store, passwordRef(original), "original-password")
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Rename(backup, path); err != nil {
				t.Fatal(err)
			}
			connections, err := editor.reload(context.Background())
			if err != nil || len(connections) != 1 || connections[0].Endpoint.Protocol != domain.ProtocolSSH {
				t.Fatalf("original config changed: %v", err)
			}
		})
	}
}

func TestPickerEditorPartialDeleteRestoresPasswords(t *testing.T) {
	editor, original := testPickerEditor(t)
	store := editor.state.secretStore
	sshRef := passwordRef(original)
	rdpRef := sshRef
	rdpRef.Protocol = secrets.ProtocolRDP
	for _, ref := range []secrets.Ref{sshRef, rdpRef} {
		if err := store.Store(context.Background(), ref, []byte("original-password")); err != nil {
			t.Fatal(err)
		}
	}
	deleteErr := errors.New("delete failed after removing item")
	editor.state.secretStore = failingSecretStore{secretStoreBackend: store, delete: func(ctx context.Context, ref secrets.Ref) error {
		if err := store.Delete(ctx, ref); err != nil {
			return err
		}
		if ref.Protocol == secrets.ProtocolRDP {
			return deleteErr
		}
		return nil
	}}
	if _, err := editor.Delete(context.Background(), original); !errors.Is(err, deleteErr) {
		t.Fatalf("Delete() error = %v", err)
	}
	assertPassword(t, store, sshRef, "original-password")
	assertPassword(t, store, rdpRef, "original-password")
	connections, err := editor.reload(context.Background())
	if err != nil || len(connections) != 1 {
		t.Fatalf("config changed on partial deletion: %v", err)
	}
}

func TestPickerEditorReportsRestoreFailure(t *testing.T) {
	editor, original := testPickerEditor(t)
	store := editor.state.secretStore
	if err := store.Store(context.Background(), passwordRef(original), []byte("original-password")); err != nil {
		t.Fatal(err)
	}
	deleteErr, restoreErr := errors.New("delete failed"), errors.New("restore failed")
	editor.state.secretStore = failingSecretStore{secretStoreBackend: store,
		delete: func(ctx context.Context, ref secrets.Ref) error { _ = store.Delete(ctx, ref); return deleteErr },
		store:  func(context.Context, secrets.Ref, []byte) error { return restoreErr },
	}
	_, err := editor.Delete(context.Background(), original)
	if !errors.Is(err, deleteErr) || !errors.Is(err, restoreErr) || strings.Contains(err.Error(), "original-password") {
		t.Fatalf("Delete() did not safely report both failures: %v", err)
	}
}

func TestPickerEditorAbsentSecrets(t *testing.T) {
	editor, original := testPickerEditor(t)
	connections, err := editor.Delete(context.Background(), original)
	if err != nil || len(connections) != 0 {
		t.Fatalf("Delete() with absent secrets: %v", err)
	}
	if _, err := editor.state.secretStore.Lookup(context.Background(), passwordRef(original)); !errors.Is(err, secrets.ErrNotFound) {
		t.Fatalf("absent secret was recreated: %v", err)
	}
}

func TestPickerEditorCanceledDeleteRestoresPassword(t *testing.T) {
	editor, original := testPickerEditor(t)
	store := editor.state.secretStore
	if err := store.Store(context.Background(), passwordRef(original), []byte("original-password")); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	editor.state.secretStore = failingSecretStore{secretStoreBackend: store, delete: func(ctx context.Context, ref secrets.Ref) error {
		if err := store.Delete(ctx, ref); err != nil {
			return err
		}
		cancel()
		return context.Canceled
	}}
	if _, err := editor.Delete(ctx, original); !errors.Is(err, context.Canceled) {
		t.Fatalf("Delete() error = %v", err)
	}
	assertPassword(t, store, passwordRef(original), "original-password")
}

func assertPassword(t *testing.T, store secrets.Store, ref secrets.Ref, want string) {
	t.Helper()
	got, err := store.Lookup(context.Background(), ref)
	defer secrets.Clear(got)
	if err != nil || string(got) != want {
		t.Fatalf("password not preserved: %v", err)
	}
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
