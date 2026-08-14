package config

import (
	"path/filepath"
	"testing"
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
