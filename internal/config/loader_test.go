package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Alurith/hoplane/internal/domain"
)

func TestLoadAndSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.yaml")
	file := NewFile()
	port := uint16(22)
	file.Connections = []Entry{{
		Name:     "nas",
		Protocol: "ssh",
		Host:     "nas.local",
		Port:     &port,
		Options: domain.Options{
			"ssh": {
				"identity_file": "~/.ssh/id_ed25519",
				"proxy_jump":    "bastion",
			},
		},
	}}

	if err := Save(path, file); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(loaded.Connections) != 1 || loaded.Connections[0].Name != "nas" {
		t.Fatalf("loaded connections = %#v", loaded.Connections)
	}
	options := loaded.Connections[0].Options["ssh"]
	if options["identity_file"] != "~/.ssh/id_ed25519" || options["proxy_jump"] != "bastion" {
		t.Fatalf("loaded options = %#v", loaded.Connections[0].Options)
	}
}

func TestLoadRejectsMultipleYAMLDocuments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	contents := "version: 1\nconnections: []\n---\nversion: 1\nconnections: []\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("Load() error = nil, want multiple-document error")
	}
}

func TestLoadMissingReturnsEmptyVersionedFile(t *testing.T) {
	file, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if file.Version != CurrentVersion || len(file.Connections) != 0 {
		t.Fatalf("file = %#v, want empty version %d file", file, CurrentVersion)
	}
}
