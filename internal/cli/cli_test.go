package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Alurith/hoplane/internal/config"
	"github.com/Alurith/hoplane/internal/output"
)

func TestAddPersistsSSHOptions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	sshPath := filepath.Join(t.TempDir(), "ssh", "config")
	if err := Execute(context.Background(), []string{
		"add", "nas", "--config", path, "--ssh-config", sshPath,
		"--protocol", "ssh", "--host", "nas.local",
		"--identity-file", "~/.ssh/id_ed25519", "--proxy-jump", "bastion",
	}, Dependencies{
		Input:  bytes.NewBuffer(nil),
		Output: &bytes.Buffer{},
		Errors: &bytes.Buffer{},
	}); err != nil {
		t.Fatalf("add error = %v", err)
	}

	file, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	options := file.Connections[0].Options["ssh"]
	if options["identity_file"] != "~/.ssh/id_ed25519" || options["proxy_jump"] != "bastion" {
		t.Fatalf("options = %#v", file.Connections[0].Options)
	}
}

func TestAddPersistsRDPOptions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	sshPath := filepath.Join(t.TempDir(), "ssh", "config")

	err := Execute(context.Background(), []string{
		"add", "office", "--config", path, "--ssh-config", sshPath,
		"--protocol", "rdp", "--host", "desktop.example.com", "--user", "alice",
		"--rdp-client", "xfreerdp", "--rdp-fullscreen", "--rdp-ignore-certificate",
	}, Dependencies{
		Input:  bytes.NewBuffer(nil),
		Output: &bytes.Buffer{},
		Errors: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("add error = %v", err)
	}

	file, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	options := file.Connections[0].Options["rdp"]
	want := map[string]string{
		"client":             "xfreerdp",
		"fullscreen":         "true",
		"ignore_certificate": "true",
	}
	if !reflect.DeepEqual(options, want) {
		t.Fatalf("options = %#v, want %#v", options, want)
	}
}

func TestAddRejectsRDPOptionsForSSH(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	sshPath := filepath.Join(t.TempDir(), "ssh", "config")

	err := Execute(context.Background(), []string{
		"add", "nas", "--config", path, "--ssh-config", sshPath,
		"--protocol", "ssh", "--host", "nas.local", "--rdp-fullscreen",
	}, Dependencies{
		Input:  bytes.NewBuffer(nil),
		Output: &bytes.Buffer{},
		Errors: &bytes.Buffer{},
	})
	if err == nil || !strings.Contains(err.Error(), `RDP options require protocol "rdp"`) {
		t.Fatalf("add error = %v, want protocol validation error", err)
	}
}

func TestAddListAndShow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	sshPath := filepath.Join(t.TempDir(), "ssh", "config")
	var outputBuffer bytes.Buffer
	dependencies := Dependencies{Input: bytes.NewBuffer(nil), Output: &outputBuffer, Errors: &bytes.Buffer{}}

	if err := Execute(context.Background(), []string{
		"add", "office", "--config", path, "--ssh-config", sshPath, "--protocol", "rdp", "--host", "desktop.example.com", "--user", "alice",
	}, dependencies); err != nil {
		t.Fatalf("add error = %v", err)
	}
	outputBuffer.Reset()

	if err := Execute(context.Background(), []string{"list", "--config", path, "--ssh-config", sshPath}, dependencies); err != nil {
		t.Fatalf("list error = %v", err)
	}
	var listed output.ListResponse
	if err := json.Unmarshal(outputBuffer.Bytes(), &listed); err != nil {
		t.Fatalf("list output is not JSON: %v", err)
	}
	if len(listed.Connections) != 1 || listed.Connections[0].Port != 3389 {
		t.Fatalf("listed = %#v", listed)
	}

	outputBuffer.Reset()
	if err := Execute(context.Background(), []string{"show", "office", "--config", path, "--ssh-config", sshPath}, dependencies); err != nil {
		t.Fatalf("show error = %v", err)
	}
	var shown output.ConnectionResponse
	if err := json.Unmarshal(outputBuffer.Bytes(), &shown); err != nil {
		t.Fatalf("show output is not JSON: %v", err)
	}
	if shown.Connection.Name != "office" {
		t.Fatalf("shown = %#v", shown)
	}
}
