package secrets

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func testRef() Ref {
	return Ref{
		ConnectionID: "0123456789abcdef0123456789abcdef",
		Protocol:     ProtocolSSH,
		Kind:         LoginPassword,
	}
}

func TestMemoryStoreRoundTripCopiesValues(t *testing.T) {
	store := NewMemoryStore()
	value := []byte("correct horse battery staple")
	if err := store.Store(context.Background(), testRef(), value); err != nil {
		t.Fatalf("Store() error = %v", err)
	}
	value[0] = 'X'
	got, err := store.Lookup(context.Background(), testRef())
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if string(got) != "correct horse battery staple" {
		t.Fatalf("Lookup() = %q", got)
	}
	got[0] = 'Y'
	present, err := store.Has(context.Background(), testRef())
	if err != nil || !present {
		t.Fatalf("Has() = %v, %v; want true", present, err)
	}
	if err := store.Delete(context.Background(), testRef()); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := store.Lookup(context.Background(), testRef()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Lookup() error = %v, want ErrNotFound", err)
	}
}

func TestValidateSecretForProtocolUsesIndependentLimits(t *testing.T) {
	if got, err := MaxSecretBytesForProtocol(ProtocolSSH); err != nil || got != MaxSSHAskpassBytes {
		t.Fatalf("SSH limit = %d, %v", got, err)
	}
	if got, err := MaxSecretBytesForProtocol(ProtocolRDP); err != nil || got != MaxRDPPasswordBytes {
		t.Fatalf("RDP limit = %d, %v", got, err)
	}
	if _, err := MaxSecretBytesForProtocol("other"); err == nil {
		t.Fatal("unsupported protocol limit was accepted")
	}

	for _, test := range []struct {
		protocol string
		length   int
		valid    bool
	}{
		{ProtocolSSH, MaxSSHAskpassBytes, true},
		{ProtocolSSH, MaxSSHAskpassBytes + 1, false},
		{ProtocolRDP, MaxRDPPasswordBytes, true},
		{ProtocolRDP, MaxRDPPasswordBytes + 1, false},
	} {
		value := bytes.Repeat([]byte{'a'}, test.length)
		err := ValidateSecretForProtocol(test.protocol, value)
		if (err == nil) != test.valid {
			t.Fatalf("ValidateSecretForProtocol(%s, %d) error = %v, valid = %v", test.protocol, test.length, err, test.valid)
		}
	}
}

func TestValidateSecretRejectsTransportDelimiters(t *testing.T) {
	for _, value := range [][]byte{nil, {}, {'a', '\n'}, {'a', '\r'}, {'a', 0}} {
		if err := ValidateSecretForProtocol(ProtocolRDP, value); err == nil {
			t.Fatalf("ValidateSecretForProtocol(%q) = nil, want error", value)
		}
	}
	if err := ValidateSecretForProtocol(ProtocolSSH, []byte(" password with spaces ")); err != nil {
		t.Fatalf("ValidateSecretForProtocol() error = %v", err)
	}
}

func TestRefRejectsInvalidConnectionID(t *testing.T) {
	ref := testRef()
	ref.ConnectionID = "office"
	if err := ref.validate(); err == nil {
		t.Fatal("validate() = nil, want invalid ID error")
	}
}
