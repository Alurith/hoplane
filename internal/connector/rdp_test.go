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

func rdpConnection() domain.Connection {
	return domain.Connection{
		Name: "office",
		Endpoint: domain.Endpoint{
			Protocol: domain.ProtocolRDP,
			Host:     "desktop.example.com",
			Port:     3389,
			User:     "alice",
		},
	}
}

func TestRDPConnectorPlansBaseInvocation(t *testing.T) {
	connector := NewRDPConnector(&fakeRunner{})

	got, err := connector.Plan(rdpConnection())
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	want := Invocation{
		Program: "xfreerdp",
		Args:    []string{"/v:desktop.example.com:3389", "/u:alice"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Plan() = %#v, want %#v", got, want)
	}
}

func TestRDPConnectorPlansWithoutUser(t *testing.T) {
	connector := NewRDPConnector(&fakeRunner{})
	connection := rdpConnection()
	connection.Endpoint.User = ""

	got, err := connector.Plan(connection)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	want := []string{"/v:desktop.example.com:3389"}
	if !reflect.DeepEqual(got.Args, want) {
		t.Fatalf("args = %#v, want %#v", got.Args, want)
	}
}

func TestRDPConnectorPlansFullscreenAndCertificate(t *testing.T) {
	connector := NewRDPConnector(&fakeRunner{})
	connection := rdpConnection()
	connection.Options = domain.Options{
		"rdp": {
			"fullscreen":         "true",
			"ignore_certificate": "true",
		},
	}

	got, err := connector.Plan(connection)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	want := []string{
		"/v:desktop.example.com:3389",
		"/u:alice",
		"/f",
		"/cert:ignore",
	}
	if !reflect.DeepEqual(got.Args, want) {
		t.Fatalf("args = %#v, want %#v", got.Args, want)
	}
}

func TestRDPConnectorPlansIPv6Host(t *testing.T) {
	connector := NewRDPConnector(&fakeRunner{})
	connection := rdpConnection()
	connection.Endpoint.Host = "2001:db8::10"

	got, err := connector.Plan(connection)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	want := "/v:[2001:db8::10]:3389"
	if got.Args[0] != want {
		t.Fatalf("target = %q, want %q", got.Args[0], want)
	}
}

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

func TestRDPConnectorRejectsUnsupportedClient(t *testing.T) {
	connection := rdpConnection()
	connection.Options = domain.Options{"rdp": {"client": "wlfreerdp"}}

	_, err := NewRDPConnector(&fakeRunner{}).Plan(connection)
	if err == nil || !strings.Contains(err.Error(), `unsupported RDP client "wlfreerdp"`) {
		t.Fatalf("Plan() error = %v, want unsupported client error", err)
	}
}

func TestRDPConnectorPlanDoesNotLookUpClient(t *testing.T) {
	runner := &fakeRunner{path: "/usr/bin/xfreerdp"}
	connector := NewRDPConnector(runner)

	if _, err := connector.Plan(rdpConnection()); err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if runner.lookedUp != "" || runner.runCalled {
		t.Fatal("Plan() touched the process runner")
	}
}

func TestRDPConnectorForwardsStreamsAndRunsProcess(t *testing.T) {
	runner := &fakeRunner{path: "/usr/bin/xfreerdp"}
	connector := NewRDPConnector(runner)
	input := strings.NewReader("stdin")
	var output bytes.Buffer
	var diagnostics bytes.Buffer
	streams := IO{Input: input, Output: &output, Errors: &diagnostics}

	if err := connector.Connect(context.Background(), rdpConnection(), streams); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if runner.lookedUp != "xfreerdp" {
		t.Fatalf("LookPath() name = %q, want xfreerdp", runner.lookedUp)
	}
	if !runner.runCalled {
		t.Fatal("Run() was not called")
	}
	if runner.invocation.Program != "/usr/bin/xfreerdp" {
		t.Fatalf("program = %q, want /usr/bin/xfreerdp", runner.invocation.Program)
	}
	if !reflect.DeepEqual(runner.invocation.Args, []string{"/v:desktop.example.com:3389", "/u:alice"}) {
		t.Fatalf("args = %#v", runner.invocation.Args)
	}
	if runner.streams.Input != input || runner.streams.Output != &output || runner.streams.Errors != &diagnostics {
		t.Fatal("connector did not forward all streams")
	}
}

func TestRDPConnectorReportsMissingClient(t *testing.T) {
	runner := &fakeRunner{lookupErr: errors.New("not found")}
	connector := NewRDPConnector(runner)

	err := connector.Connect(context.Background(), rdpConnection(), IO{})
	if !errors.Is(err, ErrClientUnavailable) || !strings.Contains(err.Error(), `required client "xfreerdp" is not available`) {
		t.Fatalf("Connect() error = %v, want missing client error", err)
	}
	if runner.runCalled {
		t.Fatal("Run() was called after missing client")
	}
}

func TestRDPConnectorReportsExecutionError(t *testing.T) {
	executionError := errors.New("session failed")
	runner := &fakeRunner{path: "/usr/bin/xfreerdp", runErr: executionError}

	err := NewRDPConnector(runner).Connect(context.Background(), rdpConnection(), IO{})
	if !errors.Is(err, executionError) || !strings.Contains(err.Error(), "run xfreerdp") {
		t.Fatalf("Connect() error = %v, want execution error", err)
	}
}

func TestRDPConnectorHonorsCanceledContext(t *testing.T) {
	runner := &fakeRunner{path: "/usr/bin/xfreerdp"}
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
