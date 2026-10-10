package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Alurith/hoplane/internal/config"
	"github.com/Alurith/hoplane/internal/connector"
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
			"domain":             "CONTOSO",
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

	const want = "dry-run: connection \"office\" would execute xfreerdp3 /v:desktop.example.com:3389 /u:alice /d:CONTOSO /f /cert:ignore\n"
	if output.String() != want {
		t.Fatalf("output = %q, want %q", output.String(), want)
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

func TestConnectReportsWorkingAndDoneStatus(t *testing.T) {
	registry, err := connector.NewRegistry(statusConnector{})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	var output bytes.Buffer
	state := &commandState{
		dependencies: Dependencies{Output: &output, Errors: &bytes.Buffer{}},
		registry:     registry,
	}

	if err := state.connectConnection(context.Background(), statusConnection(), false); err != nil {
		t.Fatalf("connectConnection() error = %v", err)
	}

	const want = "\x1b]7501;state=working:app=hoplane:title=bmFz\x07\x1b]7501;state=done:app=hoplane:title=bmFz\x07"
	if output.String() != want {
		t.Fatalf("status output = %q, want %q", output.String(), want)
	}
}

func TestConnectReportsWorkingAndErrorStatus(t *testing.T) {
	registry, err := connector.NewRegistry(statusConnector{err: errors.New("session failed")})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	var output bytes.Buffer
	state := &commandState{
		dependencies: Dependencies{Output: &output, Errors: &bytes.Buffer{}},
		registry:     registry,
	}

	if err := state.connectConnection(context.Background(), statusConnection(), false); err == nil {
		t.Fatal("connectConnection() error = nil, want session failure")
	}

	const want = "\x1b]7501;state=working:app=hoplane:title=bmFz\x07\x1b]7501;state=error:app=hoplane:title=bmFz\x07"
	if output.String() != want {
		t.Fatalf("status output = %q, want %q", output.String(), want)
	}
}

func TestConnectTruncatesOversizedTitle(t *testing.T) {
	registry, err := connector.NewRegistry(statusConnector{})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	var output bytes.Buffer
	state := &commandState{
		dependencies: Dependencies{Output: &output, Errors: &bytes.Buffer{}},
		registry:     registry,
	}
	connection := statusConnection()
	connection.Name = strings.Repeat("x", 193)

	if err := state.connectConnection(context.Background(), connection, false); err != nil {
		t.Fatalf("connectConnection() error = %v", err)
	}

	reports := strings.Split(output.String(), "\x1b]7501;")[1:]
	if len(reports) != 2 {
		t.Fatalf("status reports = %d, want 2", len(reports))
	}
	for _, report := range reports {
		report = strings.TrimSuffix(report, "\x07")
		_, encodedTitle, ok := strings.Cut(report, ":title=")
		if !ok {
			t.Fatalf("status report = %q, missing title", report)
		}
		decodedTitle, err := base64.StdEncoding.DecodeString(encodedTitle)
		if err != nil {
			t.Fatalf("title is not valid Base64: %v", err)
		}
		if len(decodedTitle) != 192 {
			t.Fatalf("decoded title length = %d, want 192", len(decodedTitle))
		}
	}
}

type statusConnector struct {
	err error
}

func (statusConnector) Protocol() domain.Protocol { return domain.ProtocolSSH }

func (statusConnector) Plan(domain.Connection) (connector.Invocation, error) {
	return connector.Invocation{}, nil
}

func (c statusConnector) Connect(context.Context, domain.Connection, connector.IO) error {
	return c.err
}

func statusConnection() domain.Connection {
	return domain.Connection{
		Name: "nas",
		Endpoint: domain.Endpoint{
			Protocol: domain.ProtocolSSH,
			Host:     "nas.local",
			Port:     22,
		},
	}
}
