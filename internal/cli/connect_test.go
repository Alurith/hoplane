package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Alurith/hoplane/internal/config"
	"github.com/Alurith/hoplane/internal/domain"
)

func TestConnectDryRun(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	file := config.NewFile()
	file.Connections = []config.Entry{{
		Name:     "nas",
		Protocol: "ssh",
		Host:     "nas.local",
		User:     "alice",
	}}
	if err := config.Save(path, file); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	var output bytes.Buffer
	dependencies := Dependencies{
		Input:  strings.NewReader(""),
		Output: &output,
		Errors: &bytes.Buffer{},
	}
	if err := Execute(context.Background(), []string{
		"connect", "nas", "--config", path, "--dry-run",
	}, dependencies); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	const want = "dry-run: connection \"nas\" would execute ssh -p 22 -l alice -- nas.local\n"
	if output.String() != want {
		t.Fatalf("output = %q, want %q", output.String(), want)
	}
}

func TestConnectRDPDryRun(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("xfreerdp3 is currently registered only on Linux")
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	file := config.NewFile()
	file.Connections = []config.Entry{{
		Name:     "office",
		Protocol: "rdp",
		Host:     "desktop.example.com",
		User:     "alice",
		Options: domain.Options{"rdp": {
			"client":             "xfreerdp3",
			"fullscreen":         "true",
			"ignore_certificate": "true",
		}},
	}}
	if err := config.Save(path, file); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	var output bytes.Buffer
	if err := Execute(context.Background(), []string{
		"connect", "office", "--config", path, "--dry-run",
	}, Dependencies{
		Input:  strings.NewReader(""),
		Output: &output,
		Errors: &bytes.Buffer{},
	}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	const want = "dry-run: connection \"office\" would execute xfreerdp3 /v:desktop.example.com:3389 /u:alice /f /cert:ignore\n"
	if output.String() != want {
		t.Fatalf("output = %q, want %q", output.String(), want)
	}
}

func TestConnectRDPDryRunRejectsUnregisteredClient(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	file := config.NewFile()
	file.Connections = []config.Entry{{
		Name:     "office",
		Protocol: "rdp",
		Host:     "desktop.example.com",
		Options:  domain.Options{"rdp": {"client": "other-client"}},
	}}
	if err := config.Save(path, file); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	err := Execute(context.Background(), []string{
		"connect", "office", "--config", path, "--dry-run",
	}, Dependencies{
		Input:  strings.NewReader(""),
		Output: &bytes.Buffer{},
		Errors: &bytes.Buffer{},
	})
	if err == nil || !strings.Contains(err.Error(), `RDP client "other-client" is not registered`) {
		t.Fatalf("Execute() error = %v, want unregistered client error", err)
	}
}

func TestConnectReportsMissingConnection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := config.Save(path, config.NewFile()); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	err := Execute(context.Background(), []string{
		"connect", "missing", "--config", path,
	}, Dependencies{
		Input:  strings.NewReader(""),
		Output: &bytes.Buffer{},
		Errors: &bytes.Buffer{},
	})
	if err == nil || !strings.Contains(err.Error(), `connection "missing" not found`) {
		t.Fatalf("Execute() error = %v, want missing connection error", err)
	}
}
