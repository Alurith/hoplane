package connector

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"os"
	"path/filepath"

	"github.com/Alurith/hoplane/internal/domain"
)

type fakeRunner struct {
	path       string
	lookupErr  error
	runErr     error
	lookedUp   string
	invocation Invocation
	streams    IO
	runCalled  bool
}

func (r *fakeRunner) LookPath(name string) (string, error) {
	r.lookedUp = name
	if r.lookupErr != nil {
		return "", r.lookupErr
	}
	return r.path, nil
}

func (r *fakeRunner) Run(_ context.Context, invocation Invocation, streams IO) error {
	r.runCalled = true
	r.invocation = invocation
	r.streams = streams
	return r.runErr
}

func TestSSHConnectorPlansInvocation(t *testing.T) {
	connector := NewSSHConnector(&fakeRunner{})
	connection := domain.Connection{
		Endpoint: domain.Endpoint{
			Protocol: domain.ProtocolSSH,
			Host:     "nas.local",
			Port:     2222,
			User:     "alice",
		},
	}

	got, err := connector.Plan(connection)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	want := Invocation{Program: "ssh", Args: []string{"-p", "2222", "alice@nas.local"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Plan() = %#v, want %#v", got, want)
	}
}

func TestSSHConnectorPlansDirectIPv6WithoutBrackets(t *testing.T) {
	connector := NewSSHConnector(&fakeRunner{})
	connection := domain.Connection{
		Endpoint: domain.Endpoint{
			Protocol: domain.ProtocolSSH,
			Host:     "2001:db8::10",
			Port:     2222,
			User:     "alice",
		},
	}

	got, err := connector.Plan(connection)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	want := Invocation{Program: "ssh", Args: []string{"-p", "2222", "alice@2001:db8::10"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Plan() = %#v, want %#v", got, want)
	}
}

func TestSSHConnectorPlansSSHOptions(t *testing.T) {
	connector := NewSSHConnector(&fakeRunner{})
	connection := domain.Connection{
		Endpoint: domain.Endpoint{
			Protocol: domain.ProtocolSSH,
			Host:     "nas.local",
			Port:     2222,
			User:     "alice",
		},
		Options: domain.Options{
			"ssh": {
				"identity_file": "/home/alice/.ssh/id_ed25519",
				"proxy_jump":    "bastion",
			},
		},
	}

	got, err := connector.Plan(connection)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	want := Invocation{
		Program: "ssh",
		Args:    []string{"-i", "/home/alice/.ssh/id_ed25519", "-J", "bastion", "-p", "2222", "alice@nas.local"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Plan() = %#v, want %#v", got, want)
	}
}

func TestSSHConnectorPlansConfigAliasWithoutOverrides(t *testing.T) {
	connector := NewSSHConnector(&fakeRunner{})
	configPath := filepath.Join(t.TempDir(), "ssh", "config")
	connection := domain.Connection{
		Endpoint: domain.Endpoint{
			Protocol: domain.ProtocolSSH,
			Host:     "nas",
			Port:     22,
		},
		Options: domain.Options{
			"ssh": {
				"config_file": configPath,
				"host_alias":  "nas",
			},
		},
	}

	got, err := connector.Plan(connection)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	want := Invocation{Program: "ssh", Args: []string{"-F", configPath, "nas"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Plan() = %#v, want %#v", got, want)
	}
}

func TestSSHConnectorExpandsIdentityFileHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot resolve home directory: %v", err)
	}
	connector := NewSSHConnector(&fakeRunner{})
	connection := domain.Connection{
		Endpoint: domain.Endpoint{Protocol: domain.ProtocolSSH, Host: "nas.local", Port: 22},
		Options:  domain.Options{"ssh": {"identity_file": "~/.ssh/id_ed25519"}},
	}

	got, err := connector.Plan(connection)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	want := filepath.Join(home, ".ssh", "id_ed25519")
	if !reflect.DeepEqual(got.Args, []string{"-i", want, "-p", "22", "nas.local"}) {
		t.Fatalf("args = %#v, want %#v", got.Args, []string{"-i", want, "-p", "22", "nas.local"})
	}
}

func TestSSHConnectorRejectsUnknownOption(t *testing.T) {
	connector := NewSSHConnector(&fakeRunner{})
	_, err := connector.Plan(domain.Connection{
		Endpoint: domain.Endpoint{Protocol: domain.ProtocolSSH, Host: "nas.local", Port: 22},
		Options:  domain.Options{"ssh": {"unknown": "value"}},
	})
	if err == nil || !strings.Contains(err.Error(), `unsupported SSH option "unknown"`) {
		t.Fatalf("Plan() error = %v, want unknown option error", err)
	}
}

func TestSSHConnectorForwardsStreamsAndRunsProcess(t *testing.T) {
	runner := &fakeRunner{path: "/usr/bin/ssh"}
	connector := NewSSHConnector(runner)
	input := strings.NewReader("stdin")
	var output bytes.Buffer
	var diagnostics bytes.Buffer
	streams := IO{Input: input, Output: &output, Errors: &diagnostics}
	connection := domain.Connection{
		Endpoint: domain.Endpoint{
			Protocol: domain.ProtocolSSH,
			Host:     "nas.local",
			Port:     22,
		},
	}

	if err := connector.Connect(context.Background(), connection, streams); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if runner.lookedUp != "ssh" {
		t.Fatalf("LookPath() name = %q, want ssh", runner.lookedUp)
	}
	if !runner.runCalled {
		t.Fatal("Run() was not called")
	}
	if runner.invocation.Program != "/usr/bin/ssh" {
		t.Fatalf("program = %q, want /usr/bin/ssh", runner.invocation.Program)
	}
	if !reflect.DeepEqual(runner.invocation.Args, []string{"-p", "22", "nas.local"}) {
		t.Fatalf("args = %#v", runner.invocation.Args)
	}
	if runner.streams.Input != input || runner.streams.Output != &output || runner.streams.Errors != &diagnostics {
		t.Fatal("connector did not forward all streams")
	}
}

func TestSSHConnectorReportsMissingClient(t *testing.T) {
	runner := &fakeRunner{lookupErr: errors.New("not found")}
	connector := NewSSHConnector(runner)
	connection := domain.Connection{
		Endpoint: domain.Endpoint{
			Protocol: domain.ProtocolSSH,
			Host:     "nas.local",
			Port:     22,
		},
	}

	err := connector.Connect(context.Background(), connection, IO{})
	if !errors.Is(err, ErrClientUnavailable) || !strings.Contains(err.Error(), `required client "ssh" is not available`) {
		t.Fatalf("Connect() error = %v, want missing client error", err)
	}
	if runner.runCalled {
		t.Fatal("Run() was called after missing client")
	}
}

func TestSSHConnectorHonorsCanceledContext(t *testing.T) {
	runner := &fakeRunner{path: "/usr/bin/ssh"}
	connector := NewSSHConnector(runner)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := connector.Connect(ctx, domain.Connection{}, IO{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Connect() error = %v, want context.Canceled", err)
	}
	if runner.lookedUp != "" || runner.runCalled {
		t.Fatal("connector touched the runner after context cancellation")
	}
}
