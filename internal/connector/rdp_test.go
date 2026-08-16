package connector

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Alurith/hoplane/internal/domain"
	"github.com/Alurith/hoplane/internal/rdpoptions"
)

type stubRDPClient struct {
	id           string
	program      string
	capabilities rdpClientCapabilities
	args         []string
	planErr      error
	planCalls    int
	options      rdpoptions.Options
}

func (c *stubRDPClient) ID() string { return c.id }

func (c *stubRDPClient) Program() string { return c.program }

func (c *stubRDPClient) Capabilities() rdpClientCapabilities { return c.capabilities }

func (c *stubRDPClient) Plan(_ domain.Connection, options rdpoptions.Options) ([]string, error) {
	c.planCalls++
	c.options = options
	return append([]string(nil), c.args...), c.planErr
}

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

func connectorWithRDPClients(
	runner ProcessRunner,
	defaultClient string,
	clients ...*stubRDPClient,
) RDPConnector {
	registered := make(map[string]rdpClient, len(clients))
	for _, client := range clients {
		registered[client.ID()] = client
	}
	return newRDPConnector(runner, defaultClient, registered)
}

func TestRDPConnectorSelectsDefaultClient(t *testing.T) {
	client := &stubRDPClient{id: "test-client", program: "test-rdp", args: []string{"server"}}
	connector := connectorWithRDPClients(&fakeRunner{}, client.ID(), client)

	got, err := connector.Plan(rdpConnection())
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	want := Invocation{Program: "test-rdp", Args: []string{"server"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Plan() = %#v, want %#v", got, want)
	}
	if client.planCalls != 1 {
		t.Fatalf("Plan() calls = %d, want 1", client.planCalls)
	}
}

func TestRDPConnectorSelectsConfiguredLogicalClient(t *testing.T) {
	defaultClient := &stubRDPClient{id: "default-client", program: "default-rdp"}
	selectedClient := &stubRDPClient{id: "selected-client", program: "selected-rdp"}
	connector := connectorWithRDPClients(&fakeRunner{}, defaultClient.ID(), defaultClient, selectedClient)
	connection := rdpConnection()
	connection.Options = domain.Options{"rdp": {"client": selectedClient.ID()}}

	got, err := connector.Plan(connection)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if got.Program != selectedClient.Program() || selectedClient.planCalls != 1 || defaultClient.planCalls != 0 {
		t.Fatalf("selection = %#v, default calls = %d, selected calls = %d", got, defaultClient.planCalls, selectedClient.planCalls)
	}
}

func TestRDPConnectorRejectsUnregisteredClient(t *testing.T) {
	client := &stubRDPClient{id: "test-client", program: "test-rdp"}
	connector := connectorWithRDPClients(&fakeRunner{}, client.ID(), client)
	connection := rdpConnection()
	connection.Options = domain.Options{"rdp": {"client": "other-client"}}

	_, err := connector.Plan(connection)
	if err == nil || !strings.Contains(err.Error(), `RDP client "other-client" is not registered`) {
		t.Fatalf("Plan() error = %v, want unregistered client error", err)
	}
	if client.planCalls != 0 {
		t.Fatal("registered adapter was used as a fallback")
	}
}

func TestRDPConnectorRejectsMissingPlatformDefault(t *testing.T) {
	_, err := newRDPConnector(&fakeRunner{}, "", nil).Plan(rdpConnection())
	if err == nil || !strings.Contains(err.Error(), "no RDP client is registered for platform") {
		t.Fatalf("Plan() error = %v, want missing platform client error", err)
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
			client := &stubRDPClient{id: "test-client", program: "test-rdp"}
			connection := rdpConnection()
			test.mutate(&connection)
			_, err := connectorWithRDPClients(&fakeRunner{}, client.ID(), client).Plan(connection)
			if err == nil || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("Plan() error = %v, want %q", err, test.message)
			}
		})
	}
}

func TestRDPConnectorRejectsUnsupportedCommonOptions(t *testing.T) {
	tests := []struct {
		option string
		value  string
	}{
		{option: rdpoptions.Fullscreen, value: "true"},
		{option: rdpoptions.IgnoreCertificate, value: "true"},
	}
	for _, test := range tests {
		t.Run(test.option, func(t *testing.T) {
			client := &stubRDPClient{id: "limited-client", program: "limited-rdp"}
			connection := rdpConnection()
			connection.Options = domain.Options{"rdp": {test.option: test.value}}

			_, err := connectorWithRDPClients(&fakeRunner{}, client.ID(), client).Plan(connection)
			if err == nil || !strings.Contains(err.Error(), `does not support option "`+test.option+`"`) {
				t.Fatalf("Plan() error = %v, want unsupported option error", err)
			}
			if client.planCalls != 0 {
				t.Fatal("adapter plan ran with an unsupported option")
			}
		})
	}
}

func TestRDPConnectorRejectsAdapterExecutablePath(t *testing.T) {
	client := &stubRDPClient{id: "unsafe-client", program: "/usr/bin/unsafe-rdp"}
	_, err := connectorWithRDPClients(&fakeRunner{}, client.ID(), client).Plan(rdpConnection())
	if err == nil || !strings.Contains(err.Error(), "invalid executable") {
		t.Fatalf("Plan() error = %v, want invalid executable error", err)
	}
}

func TestRDPConnectorPlanDoesNotLookUpClient(t *testing.T) {
	runner := &fakeRunner{lookupErr: errors.New("must not be called")}
	client := &stubRDPClient{id: "test-client", program: "test-rdp"}
	connector := connectorWithRDPClients(runner, client.ID(), client)

	if _, err := connector.Plan(rdpConnection()); err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if runner.lookedUp != "" || runner.runCalled {
		t.Fatal("Plan() touched the process runner")
	}
}

func TestRDPConnectorForwardsStreamsAndRunsProcess(t *testing.T) {
	runner := &fakeRunner{path: "/usr/bin/test-rdp"}
	client := &stubRDPClient{id: "test-client", program: "test-rdp", args: []string{"server"}}
	connector := connectorWithRDPClients(runner, client.ID(), client)
	input := strings.NewReader("stdin")
	var output bytes.Buffer
	var diagnostics bytes.Buffer
	streams := IO{Input: input, Output: &output, Errors: &diagnostics}

	if err := connector.Connect(context.Background(), rdpConnection(), streams); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if runner.lookedUp != "test-rdp" {
		t.Fatalf("LookPath() name = %q, want test-rdp", runner.lookedUp)
	}
	if !runner.runCalled {
		t.Fatal("Run() was not called")
	}
	want := Invocation{Program: "/usr/bin/test-rdp", Args: []string{"server"}}
	if !reflect.DeepEqual(runner.invocation, want) {
		t.Fatalf("invocation = %#v, want %#v", runner.invocation, want)
	}
	if runner.streams.Input != input || runner.streams.Output != &output || runner.streams.Errors != &diagnostics {
		t.Fatal("connector did not forward all streams")
	}
}

func TestRDPConnectorReportsMissingExecutable(t *testing.T) {
	runner := &fakeRunner{lookupErr: errors.New("not found")}
	client := &stubRDPClient{id: "test-client", program: "test-rdp"}
	connector := connectorWithRDPClients(runner, client.ID(), client)

	err := connector.Connect(context.Background(), rdpConnection(), IO{})
	if !errors.Is(err, ErrClientUnavailable) || !strings.Contains(err.Error(), `executable "test-rdp" for RDP client "test-client" is not available`) {
		t.Fatalf("Connect() error = %v, want missing executable error", err)
	}
	if runner.runCalled {
		t.Fatal("Run() was called after missing executable")
	}
}

func TestRDPConnectorReportsExecutionError(t *testing.T) {
	executionError := errors.New("session failed")
	runner := &fakeRunner{path: "/usr/bin/test-rdp", runErr: executionError}
	client := &stubRDPClient{id: "test-client", program: "test-rdp"}

	err := connectorWithRDPClients(runner, client.ID(), client).Connect(context.Background(), rdpConnection(), IO{})
	if !errors.Is(err, executionError) || !strings.Contains(err.Error(), "run test-rdp") {
		t.Fatalf("Connect() error = %v, want execution error", err)
	}
}

func TestRDPConnectorHonorsCanceledContext(t *testing.T) {
	runner := &fakeRunner{path: "/usr/bin/test-rdp"}
	client := &stubRDPClient{id: "test-client", program: "test-rdp"}
	connector := connectorWithRDPClients(runner, client.ID(), client)
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
