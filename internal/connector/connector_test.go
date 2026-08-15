package connector

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/Alurith/hoplane/internal/domain"
)

type fakeConnector struct {
	protocol domain.Protocol
	calls    int
}

func (f *fakeConnector) Protocol() domain.Protocol {
	return f.protocol
}

func (f *fakeConnector) Connect(context.Context, domain.Connection, IO) error {
	f.calls++
	return nil
}

func TestRegistryLooksUpConnectorByProtocol(t *testing.T) {
	ssh := &fakeConnector{protocol: domain.ProtocolSSH}
	registry, err := NewRegistry(ssh)
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	got, err := registry.Lookup(domain.ProtocolSSH)
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if got != ssh {
		t.Fatalf("Lookup() = %T, want registered connector", got)
	}
}

func TestDefaultRegistryIncludesRDP(t *testing.T) {
	registry := DefaultRegistry()

	got, err := registry.Lookup(domain.ProtocolRDP)
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if got.Protocol() != domain.ProtocolRDP {
		t.Fatalf("connector protocol = %q, want %q", got.Protocol(), domain.ProtocolRDP)
	}
}

func TestRegistryRejectsUnsupportedProtocol(t *testing.T) {
	registry, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	_, err = registry.Lookup(domain.Protocol("custom"))
	if !errors.Is(err, ErrUnsupportedProtocol) {
		t.Fatalf("Lookup() error = %v, want ErrUnsupportedProtocol", err)
	}
}

func TestRegistryRejectsDuplicateProtocols(t *testing.T) {
	_, err := NewRegistry(
		&fakeConnector{protocol: domain.ProtocolSSH},
		&fakeConnector{protocol: domain.ProtocolSSH},
	)
	if err == nil {
		t.Fatal("NewRegistry() error = nil, want duplicate protocol error")
	}
}

func TestRegistryDryRunUsesPlannerWithoutExecutingConnector(t *testing.T) {
	registry, err := NewRegistry(NewSSHConnector(&fakeRunner{path: "/usr/bin/ssh"}))
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	var output bytes.Buffer
	connection := domain.Connection{
		Name: "nas",
		Endpoint: domain.Endpoint{
			Protocol: domain.ProtocolSSH,
			Host:     "nas.local",
			Port:     22,
		},
	}
	if err := registry.Connect(
		context.Background(),
		connection,
		IO{Output: &output},
		ConnectOptions{DryRun: true},
	); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	const want = "dry-run: connection \"nas\" would execute ssh -p 22 -- nas.local\n"
	if output.String() != want {
		t.Fatalf("output = %q, want %q", output.String(), want)
	}
}

func TestRegistryDryRunRDPUsesXFREERDP(t *testing.T) {
	registry, err := NewRegistry(NewRDPConnector(&fakeRunner{path: "/usr/bin/xfreerdp"}))
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	var output bytes.Buffer
	connection := domain.Connection{
		Name: "office",
		Endpoint: domain.Endpoint{
			Protocol: domain.ProtocolRDP,
			Host:     "desktop.example.com",
			Port:     3389,
			User:     "alice",
		},
	}
	if err := registry.Connect(
		context.Background(),
		connection,
		IO{Output: &output},
		ConnectOptions{DryRun: true},
	); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}

	const want = "dry-run: connection \"office\" would execute xfreerdp /v:desktop.example.com:3389 /u:alice\n"
	if output.String() != want {
		t.Fatalf("output = %q, want %q", output.String(), want)
	}
}

func TestRegistryDryRunRequiresPlanner(t *testing.T) {
	registry, err := NewRegistry(&fakeConnector{protocol: domain.ProtocolSSH})
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}

	err = registry.Connect(
		context.Background(),
		domain.Connection{Endpoint: domain.Endpoint{Protocol: domain.ProtocolSSH}},
		IO{},
		ConnectOptions{DryRun: true},
	)
	if !errors.Is(err, ErrDryRunUnsupported) {
		t.Fatalf("Connect() error = %v, want ErrDryRunUnsupported", err)
	}
}
