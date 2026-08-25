package connector

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/Alurith/hoplane/internal/domain"
	"github.com/Alurith/hoplane/internal/secrets"
	"github.com/Alurith/hoplane/internal/terminal"
)

var (
	ErrUnsupportedProtocol = errors.New("unsupported protocol")
	ErrClientUnavailable   = errors.New("required client unavailable")
	ErrSecureTransport     = errors.New("secure secret transport unavailable")
)

// IO contains the streams a connector may use for an interactive connection.
type IO struct {
	Input  io.Reader
	Output io.Writer
	Errors io.Writer
}

// Connector executes a connection for one protocol.
type Connector interface {
	Protocol() domain.Protocol
	Plan(domain.Connection) (Invocation, error)
	Connect(context.Context, domain.Connection, IO) error
}

// Invocation describes an external process and its arguments.
type Invocation struct {
	Program string
	Args    []string
}

func (i Invocation) String() string {
	parts := make([]string, 0, len(i.Args)+1)
	parts = append(parts, shellQuote(i.Program))
	for _, arg := range i.Args {
		parts = append(parts, shellQuote(arg))
	}
	return strings.Join(parts, " ")
}

func shellQuote(value string) string {
	value = terminal.EscapeControls(value)
	if value != "" {
		needsQuote := false
		for _, r := range value {
			if unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("@%+,-./:=_", r) {
				continue
			}
			needsQuote = true
			break
		}
		if !needsQuote {
			return value
		}
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

// Registry resolves connectors by the protocol of a normalized endpoint.
type Registry struct {
	connectors map[domain.Protocol]Connector
}

func NewRegistry(connectors ...Connector) (Registry, error) {
	registered := make(map[domain.Protocol]Connector, len(connectors))
	for _, connector := range connectors {
		if connector == nil {
			return Registry{}, fmt.Errorf("connector cannot be nil")
		}
		protocol := connector.Protocol()
		if protocol == "" {
			return Registry{}, fmt.Errorf("connector protocol cannot be empty")
		}
		if _, exists := registered[protocol]; exists {
			return Registry{}, fmt.Errorf("connector for protocol %q already registered", protocol)
		}
		registered[protocol] = connector
	}
	return Registry{connectors: registered}, nil
}

func DefaultRegistry(store ...secrets.Store) Registry {
	var secretStore secrets.Store
	if len(store) > 0 {
		secretStore = store[0]
	}
	if secretStore == nil {
		secretStore = secrets.NewDefaultStore()
	}
	return Registry{connectors: map[domain.Protocol]Connector{
		domain.ProtocolSSH: NewSSHConnector(ExecRunner{}, secretStore),
		domain.ProtocolRDP: NewRDPConnector(ExecRunner{}, secretStore),
	}}
}

func (r Registry) Lookup(protocol domain.Protocol) (Connector, error) {
	connector, ok := r.connectors[protocol]
	if !ok {
		return nil, fmt.Errorf("%w %q", ErrUnsupportedProtocol, protocol)
	}
	return connector, nil
}

func (r Registry) Connect(
	ctx context.Context,
	connection domain.Connection,
	streams IO,
	dryRun bool,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	connector, err := r.Lookup(connection.Endpoint.Protocol)
	if err != nil {
		return err
	}

	if dryRun {
		invocation, err := connector.Plan(connection)
		if err != nil {
			return fmt.Errorf("plan connection %q: %w", connection.Name, err)
		}
		if _, err := fmt.Fprintf(
			streams.Output,
			"dry-run: connection %q would execute %s\n",
			connection.Name,
			invocation,
		); err != nil {
			return fmt.Errorf("write dry-run: %w", err)
		}
		return nil
	}

	if err := connector.Connect(ctx, connection, streams); err != nil {
		return fmt.Errorf("connector %q: %w", connector.Protocol(), err)
	}
	return nil
}
