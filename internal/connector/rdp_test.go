//go:build linux

package connector

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Alurith/hoplane/internal/domain"
)

func TestRDPConnectorRejectsInvalidConnection(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*domain.Connection)
		message string
	}{
		{
			name:    "protocol",
			mutate:  func(connection *domain.Connection) { connection.Endpoint.Protocol = domain.ProtocolSSH },
			message: "cannot handle protocol",
		},
		{
			name:    "host",
			mutate:  func(connection *domain.Connection) { connection.Endpoint.Host = "" },
			message: "host cannot be empty",
		},
		{
			name:    "port",
			mutate:  func(connection *domain.Connection) { connection.Endpoint.Port = 0 },
			message: "port cannot be zero",
		},
		{
			name: "namespace",
			mutate: func(connection *domain.Connection) {
				connection.Options = domain.Options{"ssh": {"option": "value"}}
			},
			message: `options namespace "ssh" is not valid for RDP`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			connection := rdpConnection()
			test.mutate(&connection)
			_, err := NewRDPConnector(&fakeRunner{}).Plan(connection)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("Plan() error = %v, want %q", err, test.message)
			}
		})
	}
}

func TestRDPConnectorForwardsStreamsAndRunsProcess(t *testing.T) {
	runner := &fakeRunner{path: "/usr/bin/xfreerdp3"}
	connector := NewRDPConnector(runner)
	input := strings.NewReader("stdin")
	var output bytes.Buffer
	var diagnostics bytes.Buffer
	streams := IO{Input: input, Output: &output, Errors: &diagnostics}

	if err := connector.Connect(context.Background(), rdpConnection(), streams); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if runner.lookedUp != "xfreerdp3" {
		t.Fatalf("LookPath() name = %q, want xfreerdp3", runner.lookedUp)
	}
	if !runner.runCalled {
		t.Fatal("Run() was not called")
	}
	want := Invocation{
		Program: "/usr/bin/xfreerdp3",
		Args:    []string{"/v:desktop.example.com:3389", "/u:alice"},
	}
	if !reflect.DeepEqual(runner.invocation, want) {
		t.Fatalf("invocation = %#v, want %#v", runner.invocation, want)
	}
	if runner.streams.Input != input || runner.streams.Output != &output || runner.streams.Errors != &diagnostics {
		t.Fatal("connector did not forward all streams")
	}
}

func TestRDPConnectorReportsMissingExecutable(t *testing.T) {
	runner := &fakeRunner{lookupErr: errors.New("not found")}
	err := NewRDPConnector(runner).Connect(context.Background(), rdpConnection(), IO{})
	if !errors.Is(err, ErrClientUnavailable) || !strings.Contains(err.Error(), `required client "xfreerdp3" is not available`) {
		t.Fatalf("Connect() error = %v, want missing xfreerdp3 error", err)
	}
	if runner.runCalled {
		t.Fatal("Run() was called after missing executable")
	}
}

func TestRDPConnectorReportsExecutionError(t *testing.T) {
	executionError := errors.New("session failed")
	runner := &fakeRunner{path: "/usr/bin/xfreerdp3", runErr: executionError}

	err := NewRDPConnector(runner).Connect(context.Background(), rdpConnection(), IO{})
	if !errors.Is(err, executionError) || !strings.Contains(err.Error(), "run xfreerdp3") {
		t.Fatalf("Connect() error = %v, want execution error", err)
	}
}

func TestRDPConnectorHonorsCanceledContext(t *testing.T) {
	runner := &fakeRunner{path: "/usr/bin/xfreerdp3"}
	connector := NewRDPConnector(runner)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := connector.Connect(ctx, rdpConnection(), IO{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Connect() error = %v, want context.Canceled", err)
	}
	if runner.lookedUp != "" || runner.runCalled {
		t.Fatal("connector touched the runner after context cancellation")
	}
}
