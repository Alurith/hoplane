// Package secrets stores connection credentials outside Hoplane's configuration.
package secrets

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/Alurith/hoplane/internal/domain"
)

const (
	MaxSSHAskpassBytes  = 1023
	MaxRDPPasswordBytes = 1 << 20

	ProtocolSSH   = "ssh"
	ProtocolRDP   = "rdp"
	LoginPassword = "login-password"
)

var (
	ErrNotFound    = errors.New("secret not found")
	ErrUnavailable = errors.New("secret backend unavailable")
	ErrTimeout     = errors.New("secret backend timed out")
)

// Ref identifies one secret without containing its value.
type Ref struct {
	ConnectionID string
	Protocol     string
	Kind         string
}

func (r Ref) validate() error {
	if err := domain.ValidateConnectionID(r.ConnectionID); err != nil {
		return err
	}
	if r.Protocol != ProtocolSSH && r.Protocol != ProtocolRDP {
		return fmt.Errorf("unsupported secret protocol %q", r.Protocol)
	}
	if r.Kind != LoginPassword {
		return fmt.Errorf("unsupported secret kind %q", r.Kind)
	}
	return nil
}

// Store is the protocol-neutral secret storage contract. Implementations must
// never include secret values in returned errors.
type Store interface {
	Lookup(context.Context, Ref) ([]byte, error)
	Store(context.Context, Ref, []byte) error
	Delete(context.Context, Ref) error
	Has(context.Context, Ref) (bool, error)
}

// MaxSecretBytesForProtocol returns the largest value supported by a
// protocol's password transport.
func MaxSecretBytesForProtocol(protocol string) (int, error) {
	switch protocol {
	case ProtocolSSH:
		return MaxSSHAskpassBytes, nil
	case ProtocolRDP:
		return MaxRDPPasswordBytes, nil
	default:
		return 0, fmt.Errorf("unsupported secret protocol %q", protocol)
	}
}

// ValidateSecretForProtocol rejects values that cannot be transported safely
// by the selected protocol's password channel.
func ValidateSecretForProtocol(protocol string, value []byte) error {
	max, err := MaxSecretBytesForProtocol(protocol)
	if err != nil {
		return err
	}
	if len(value) == 0 {
		return fmt.Errorf("secret cannot be empty")
	}
	if len(value) > max {
		return fmt.Errorf("secret is too large for %s", protocol)
	}
	if bytes.IndexByte(value, 0) >= 0 {
		return fmt.Errorf("secret contains a NUL character")
	}
	if bytes.IndexByte(value, '\n') >= 0 || bytes.IndexByte(value, '\r') >= 0 {
		return fmt.Errorf("secret cannot contain newline characters")
	}
	return nil
}

// Clear overwrites a byte slice as a best-effort reduction of its lifetime in
// memory. Go cannot guarantee when all copies are collected.
func Clear(value []byte) {
	for index := range value {
		value[index] = 0
	}
}

func key(ref Ref) string {
	return strings.Join([]string{ref.ConnectionID, ref.Protocol, ref.Kind}, "\x00")
}

type memoryStore struct {
	mu     sync.RWMutex
	values map[string][]byte
}

// NewMemoryStore is used by tests and embedders that deliberately do not need
// OS-backed persistence.
func NewMemoryStore() Store {
	return &memoryStore{values: make(map[string][]byte)}
}

func (s *memoryStore) Lookup(ctx context.Context, ref Ref) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := ref.validate(); err != nil {
		return nil, err
	}
	s.mu.RLock()
	value, ok := s.values[key(ref)]
	copyValue := append([]byte(nil), value...)
	s.mu.RUnlock()
	if !ok {
		return nil, ErrNotFound
	}
	if err := ValidateSecretForProtocol(ref.Protocol, copyValue); err != nil {
		Clear(copyValue)
		return nil, fmt.Errorf("stored secret is invalid: %w", err)
	}
	return copyValue, nil
}

func (s *memoryStore) Store(ctx context.Context, ref Ref, value []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := ref.validate(); err != nil {
		return err
	}
	if err := ValidateSecretForProtocol(ref.Protocol, value); err != nil {
		return err
	}
	copyValue := append([]byte(nil), value...)
	s.mu.Lock()
	old := s.values[key(ref)]
	s.values[key(ref)] = copyValue
	s.mu.Unlock()
	Clear(old)
	return nil
}

func (s *memoryStore) Delete(ctx context.Context, ref Ref) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := ref.validate(); err != nil {
		return err
	}
	s.mu.Lock()
	old := s.values[key(ref)]
	delete(s.values, key(ref))
	s.mu.Unlock()
	Clear(old)
	return nil
}

func (s *memoryStore) Has(ctx context.Context, ref Ref) (bool, error) {
	value, err := s.Lookup(ctx, ref)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	Clear(value)
	return true, nil
}
