package connector

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/Alurith/hoplane/internal/domain"
	"github.com/Alurith/hoplane/internal/terminal"
)

var (
	ErrUnsupportedProtocol = errors.New("unsupported protocol")
	ErrDryRunUnsupported   = errors.New("dry-run unsupported")
	ErrClientUnavailable   = errors.New("required client unavailable")
)

// IO contains the streams a connector may use for an interactive connection.
type IO struct {
	Input  io.Reader
	Output io.Writer
	Errors io.Writer
}

type ConnectOptions struct {
	DryRun bool
}

// Connector executes a connection for one protocol.
type Connector interface {
	Protocol() domain.Protocol
	Connect(context.Context, domain.Connection, IO) error
}

// Planner is optionally implemented by connectors that can describe their
// process invocation without executing it.
type Planner interface {
	Plan(domain.Connection) (Invocation, error)
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

func DefaultRegistry() Registry {
	return Registry{connectors: map[domain.Protocol]Connector{
		domain.ProtocolSSH: NewSSHConnector(ExecRunner{}),
		domain.ProtocolRDP: NewRDPConnector(ExecRunner{}),
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
	options ConnectOptions,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	connector, err := r.Lookup(connection.Endpoint.Protocol)
	if err != nil {
		return err
	}

	if options.DryRun {
		planner, ok := connector.(Planner)
		if !ok {
			return fmt.Errorf("%w for protocol %q", ErrDryRunUnsupported, connector.Protocol())
		}
		invocation, err := planner.Plan(connection)
		if err != nil {
			return fmt.Errorf("plan connection %q: %w", connection.Name, err)
		}
		return WriteDryRun(streams.Output, connection, invocation)
	}

	if err := connector.Connect(ctx, connection, streams); err != nil {
		return fmt.Errorf("connector %q: %w", connector.Protocol(), err)
	}
	return nil
}
