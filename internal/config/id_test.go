package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadRejectsVersionThreeConnectionWithoutID(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("version: 3\nconnections:\n  - name: office\n    protocol: ssh\n    host: example.com\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "connection ID") {
		t.Fatalf("Load() error = %v, want missing ID error", err)
	}
}

func TestSaveGeneratesMissingIDsForNewEntries(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	file := NewFile()
	file.Connections = []Entry{{Name: "office", Protocol: "ssh", Host: "example.com"}}
	if err := Save(path, file); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if loaded.Connections[0].ID == "" {
		t.Fatal("Save() did not persist a generated ID")
	}
}
