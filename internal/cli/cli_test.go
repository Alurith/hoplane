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

func TestAddPersistsStandardSSH(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := Execute(context.Background(), []string{
		"add", "nas", "--config", path,
		"--protocol", "ssh", "--host", "nas.local", "--user", "alice",
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
	if options := file.Connections[0].Options; len(options) != 0 {
		t.Fatalf("SSH options = %#v, want none", options)
	}
}

func TestAddPersistsRDPOptions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	err := Execute(context.Background(), []string{
		"add", "office", "--config", path,
		"--protocol", "rdp", "--host", "desktop.example.com", "--user", "alice",
		"--rdp-client", "xfreerdp3", "--rdp-domain", "CONTOSO", "--rdp-fullscreen", "--rdp-ignore-certificate",
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
		"client":             "xfreerdp3",
		"domain":             "CONTOSO",
		"fullscreen":         "true",
		"ignore_certificate": "true",
	}
	if !reflect.DeepEqual(options, want) {
		t.Fatalf("options = %#v, want %#v", options, want)
	}
}

func TestAddRejectsEmptyRDPDomain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	err := Execute(context.Background(), []string{
		"add", "office", "--config", path,
		"--protocol", "rdp", "--host", "desktop.example.com", "--rdp-domain", " ",
	}, Dependencies{
		Input:  bytes.NewBuffer(nil),
		Output: &bytes.Buffer{},
		Errors: &bytes.Buffer{},
	})
	if err == nil || !strings.Contains(err.Error(), `RDP option "domain" cannot be empty`) {
		t.Fatalf("add error = %v, want empty domain error", err)
	}
}

func TestAddRejectsExecutableLikeRDPClient(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	err := Execute(context.Background(), []string{
		"add", "office", "--config", path,
		"--protocol", "rdp", "--host", "desktop.example.com", "--rdp-client", "/usr/bin/xfreerdp3",
	}, Dependencies{
		Input:  bytes.NewBuffer(nil),
		Output: &bytes.Buffer{},
		Errors: &bytes.Buffer{},
	})
	if err == nil || !strings.Contains(err.Error(), "must be a logical client ID") {
		t.Fatalf("add error = %v, want logical client ID rejection", err)
	}
}

func TestAddRejectsObsoleteSSHFlag(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	err := Execute(context.Background(), []string{
		"add", "nas", "--config", path,
		"--protocol", "ssh", "--host", "nas.local", "--identity-file", "/tmp/id",
	}, Dependencies{
		Input:  bytes.NewBuffer(nil),
		Output: &bytes.Buffer{},
		Errors: &bytes.Buffer{},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("add error = %v, want obsolete flag rejection", err)
	}
}

func TestAddRejectsRDPOptionsForSSH(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	err := Execute(context.Background(), []string{
		"add", "nas", "--config", path,
		"--protocol", "ssh", "--host", "nas.local", "--rdp-domain", "CONTOSO",
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
	var outputBuffer bytes.Buffer
	dependencies := Dependencies{Input: bytes.NewBuffer(nil), Output: &outputBuffer, Errors: &bytes.Buffer{}}

	if err := Execute(context.Background(), []string{
		"add", "office", "--config", path, "--protocol", "rdp", "--host", "desktop.example.com", "--user", "alice",
	}, dependencies); err != nil {
		t.Fatalf("add error = %v", err)
	}
	outputBuffer.Reset()

	if err := Execute(context.Background(), []string{"list", "--config", path}, dependencies); err != nil {
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
	if err := Execute(context.Background(), []string{"show", "office", "--config", path}, dependencies); err != nil {
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
