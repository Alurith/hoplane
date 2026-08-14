package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Alurith/hoplane/internal/config"
	"github.com/Alurith/hoplane/internal/connector"
)

func TestConnectDryRun(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	sshPath := filepath.Join(t.TempDir(), "ssh", "config")
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
		"connect", "nas", "--config", path, "--ssh-config", sshPath, "--dry-run",
	}, dependencies); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	const want = "dry-run: connection \"nas\" would execute ssh -p 22 alice@nas.local\n"
	if output.String() != want {
		t.Fatalf("output = %q, want %q", output.String(), want)
	}
}

func TestConnectSSHConfigAliasDryRun(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	sshPath := filepath.Join(t.TempDir(), "ssh", "config")
	if err := config.Save(path, config.NewFile()); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(sshPath), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(sshPath, []byte("Host nas\n    HostName nas.internal\n    Port 2222\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var output bytes.Buffer
	if err := Execute(context.Background(), []string{
		"connect", "nas", "--config", path, "--ssh-config", sshPath, "--dry-run",
	}, Dependencies{
		Input:  strings.NewReader(""),
		Output: &output,
		Errors: &bytes.Buffer{},
	}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	want := "dry-run: connection \"nas\" would execute ssh -F " + sshPath + " nas\n"
	if output.String() != want {
		t.Fatalf("output = %q, want %q", output.String(), want)
	}
}

func TestConnectRejectsUnsupportedProtocol(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	sshPath := filepath.Join(t.TempDir(), "ssh", "config")
	file := config.NewFile()
	port := uint16(3389)
	file.Connections = []config.Entry{{
		Name:     "office",
		Protocol: "rdp",
		Host:     "desktop.local",
		Port:     &port,
	}}
	if err := config.Save(path, file); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	err := Execute(context.Background(), []string{
		"connect", "office", "--config", path, "--ssh-config", sshPath,
	}, Dependencies{
		Input:  strings.NewReader(""),
		Output: &bytes.Buffer{},
		Errors: &bytes.Buffer{},
	})
	if !errors.Is(err, connector.ErrUnsupportedProtocol) {
		t.Fatalf("Execute() error = %v, want ErrUnsupportedProtocol", err)
	}
}

func TestConnectReportsMissingConnection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	sshPath := filepath.Join(t.TempDir(), "ssh", "config")
	if err := config.Save(path, config.NewFile()); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	err := Execute(context.Background(), []string{
		"connect", "missing", "--config", path, "--ssh-config", sshPath,
	}, Dependencies{
		Input:  strings.NewReader(""),
		Output: &bytes.Buffer{},
		Errors: &bytes.Buffer{},
	})
	if err == nil || !strings.Contains(err.Error(), `connection "missing" not found`) {
		t.Fatalf("Execute() error = %v, want missing connection error", err)
	}
}
