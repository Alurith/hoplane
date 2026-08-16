package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Alurith/hoplane/internal/domain"
)

func TestLoadAndSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.yaml")
	file := NewFile()
	port := uint16(3389)
	file.Connections = []Entry{{
		Name:     "office",
		Protocol: "rdp",
		Host:     "desktop.example.com",
		Port:     &port,
		Options: domain.Options{
			"rdp": {
				"client":     "xfreerdp3",
				"fullscreen": "true",
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
	if len(loaded.Connections) != 1 || loaded.Connections[0].Name != "office" {
		t.Fatalf("loaded connections = %#v", loaded.Connections)
	}
	options := loaded.Connections[0].Options["rdp"]
	if options["client"] != "xfreerdp3" || options["fullscreen"] != "true" {
		t.Fatalf("loaded options = %#v", loaded.Connections[0].Options)
	}
}

func TestSaveRejectsMissingVersionWithoutFallback(t *testing.T) {
	err := Save(filepath.Join(t.TempDir(), "config.yaml"), File{Connections: []Entry{}})
	if err == nil || !strings.Contains(err.Error(), "unsupported config version 0") {
		t.Fatalf("Save() error = %v, want missing version rejection", err)
	}
}

func TestLoadRejectsVersionOneWithoutMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	contents := "version: 1\nconnections: []\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "unsupported version 1, want 2") {
		t.Fatalf("Load() error = %v, want version rejection", err)
	}
}

func TestLoadRejectsMultipleYAMLDocuments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	contents := "version: 2\nconnections: []\n---\nversion: 2\nconnections: []\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("Load() error = nil, want multiple-document error")
	}
}

func TestLoadRejectsObsoleteSSHOptions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	contents := "version: 2\nconnections:\n  - name: legacy\n    protocol: ssh\n    host: example.com\n    options:\n      ssh:\n        identity_file: /tmp/id\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "SSH options are not supported") {
		t.Fatalf("Load() error = %v, want obsolete SSH options rejection", err)
	}
}

func TestLoadRejectsRDPOptionsForSSH(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	contents := "version: 2\nconnections:\n  - name: nas\n    protocol: ssh\n    host: nas.local\n    options:\n      rdp:\n        fullscreen: \"true\"\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "SSH options are not supported") {
		t.Fatalf("Load() error = %v, want incompatible options rejection", err)
	}
}

func TestLoadRejectsNonRDPNamespace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	contents := "version: 2\nconnections:\n  - name: office\n    protocol: rdp\n    host: desktop.example.com\n    options:\n      ssh:\n        option: value\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), `options namespace "ssh" is not valid for RDP`) {
		t.Fatalf("Load() error = %v, want incompatible namespace rejection", err)
	}
}

func TestLoadRejectsExecutableLikeRDPClient(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	contents := "version: 2\nconnections:\n  - name: office\n    protocol: rdp\n    host: desktop.example.com\n    options:\n      rdp:\n        client: /usr/bin/xfreerdp3\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), "must be a logical client ID") {
		t.Fatalf("Load() error = %v, want logical client ID rejection", err)
	}
}

func TestLoadRejectsUnsupportedProtocol(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	contents := "version: 2\nconnections:\n  - name: custom\n    protocol: vnc\n    host: example.com\n    port: 5900\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), `unsupported protocol "vnc"`) {
		t.Fatalf("Load() error = %v, want unsupported protocol rejection", err)
	}
}

func TestSaveRejectsArbitraryRDPCommandOptions(t *testing.T) {
	for _, option := range []string{"extra_args", "path", "program"} {
		t.Run(option, func(t *testing.T) {
			file := NewFile()
			file.Connections = []Entry{{
				Name:     "office",
				Protocol: "rdp",
				Host:     "desktop.example.com",
				Options:  domain.Options{"rdp": {option: "arbitrary"}},
			}}
			if err := Save(filepath.Join(t.TempDir(), "config.yaml"), file); err == nil || !strings.Contains(err.Error(), "unsupported RDP option") {
				t.Fatalf("Save() error = %v, want arbitrary command option rejection", err)
			}
		})
	}
}

func TestSaveRejectsSensitiveOptions(t *testing.T) {
	file := NewFile()
	file.Connections = []Entry{{
		Name:     "secret",
		Protocol: "rdp",
		Host:     "example.com",
		Options:  domain.Options{"rdp": {"api_token": "secret"}},
	}}
	if err := Save(filepath.Join(t.TempDir(), "config.yaml"), file); err == nil {
		t.Fatal("Save() error = nil, want sensitive option rejection")
	}
}

func TestSaveRejectsOversizedConfig(t *testing.T) {
	file := NewFile()
	file.Connections = []Entry{{Name: "large", Protocol: "ssh", Host: "example.com", Description: strings.Repeat("x", int(MaxBytes))}}
	if err := Save(filepath.Join(t.TempDir(), "config.yaml"), file); err == nil {
		t.Fatal("Save() error = nil, want size limit error")
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
