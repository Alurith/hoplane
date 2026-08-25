package connector

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"

	"github.com/Alurith/hoplane/internal/domain"
	"github.com/Alurith/hoplane/internal/secrets"
)

func sshTestConnection() domain.Connection {
	return domain.Connection{Endpoint: domain.Endpoint{
		Protocol: domain.ProtocolSSH,
		Host:     "nas.local",
		Port:     22,
	}}
}

type testSecretStore struct {
	value       []byte
	lookupErr   error
	lookupCalls int
}

func (s *testSecretStore) Lookup(context.Context, secrets.Ref) ([]byte, error) {
	s.lookupCalls++
	if s.lookupErr != nil {
		return nil, s.lookupErr
	}
	return append([]byte(nil), s.value...), nil
}

func (*testSecretStore) Store(context.Context, secrets.Ref, []byte) error { return nil }
func (*testSecretStore) Delete(context.Context, secrets.Ref) error        { return nil }
func (s *testSecretStore) Has(context.Context, secrets.Ref) (bool, error) {
	return s.value != nil, nil
}

func TestSSHUnavailableBackendDoesNotRunClient(t *testing.T) {
	runner := &fakeRunner{path: "/usr/bin/ssh"}
	store := &testSecretStore{lookupErr: secrets.ErrUnavailable}
	connection := sshTestConnection()
	connection.ID = "0123456789abcdef0123456789abcdef"

	err := NewSSHConnector(runner, store).Connect(context.Background(), connection, IO{})
	if !errors.Is(err, secrets.ErrUnavailable) {
		t.Fatalf("Connect() error = %v, want ErrUnavailable", err)
	}
	if runner.runCalled || runner.secureCalled {
		t.Fatal("runner executed after unavailable keyring")
	}
}

func TestRDPUnavailableBackendDoesNotRunClient(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("xfreerdp3 is currently registered only on Linux")
	}
	runner := &fakeRunner{path: "/usr/bin/xfreerdp3"}
	store := &testSecretStore{lookupErr: secrets.ErrUnavailable}
	connection := rdpConnection()
	connection.ID = "0123456789abcdef0123456789abcdef"

	err := NewRDPConnector(runner, store).Connect(context.Background(), connection, IO{})
	if !errors.Is(err, secrets.ErrUnavailable) {
		t.Fatalf("Connect() error = %v, want ErrUnavailable", err)
	}
	if runner.runCalled || runner.secureCalled {
		t.Fatal("runner executed after unavailable keyring")
	}
}

func TestSSHInvalidStoredValueDoesNotFallbackToNativePrompt(t *testing.T) {
	runner := &fakeRunner{path: "/usr/bin/ssh"}
	store := &testSecretStore{value: []byte("invalid\n")}
	connection := sshTestConnection()
	connection.ID = "0123456789abcdef0123456789abcdef"

	err := NewSSHConnector(runner, store).Connect(context.Background(), connection, IO{})
	if err == nil || strings.Contains(err.Error(), "not found") {
		t.Fatalf("Connect() error = %v, want invalid stored value error", err)
	}
	if runner.runCalled || runner.secureCalled {
		t.Fatal("runner executed after invalid stored value")
	}
}

func TestRDPArgsPayloadValidatesFramingAndIndependentLimit(t *testing.T) {
	payload := rdpArgsPayload([]string{"/v:host", "/u:alice"}, []byte("password with spaces"))
	if err := validateRDPArgsPayload(payload); err != nil {
		t.Fatalf("validateRDPArgsPayload() error = %v", err)
	}
	for _, password := range [][]byte{{'a', '\r'}, {'a', '\n'}, {'a', 0}} {
		if err := secrets.ValidateSecretForProtocol(secrets.ProtocolRDP, password); err == nil {
			t.Fatalf("RDP password %q was accepted", password)
		}
	}
	if err := validateRDPArgs([]string{"/v:host\n/p:injected"}); err == nil {
		t.Fatal("RDP argument with newline was accepted")
	}
}
