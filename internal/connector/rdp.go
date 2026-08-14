package connector

import (
	"context"
	"fmt"

	"github.com/Alurith/hoplane/internal/domain"
	"github.com/Alurith/hoplane/internal/rdpoptions"
)

// RDPConnector is a process-backed RDP connector. RDP-specific configuration
// remains in the client adapter rather than in the common domain model.
type RDPConnector struct {
	runner ProcessRunner
	client rdpClient
}

type rdpClient interface {
	Name() string
	Plan(domain.Connection, rdpoptions.Options) (Invocation, error)
}

func NewRDPConnector(runner ProcessRunner) RDPConnector {
	if runner == nil {
		runner = ExecRunner{}
	}
	return RDPConnector{runner: runner, client: xfreerdpClient{}}
}

func (RDPConnector) Protocol() domain.Protocol {
	return domain.ProtocolRDP
}

func (c RDPConnector) Plan(connection domain.Connection) (Invocation, error) {
	if connection.Endpoint.Protocol != c.Protocol() {
		return Invocation{}, fmt.Errorf(
			"RDP connector cannot handle protocol %q",
			connection.Endpoint.Protocol,
		)
	}
	if connection.Endpoint.Host == "" {
		return Invocation{}, fmt.Errorf("RDP connection host cannot be empty")
	}
	if connection.Endpoint.Port == 0 {
		return Invocation{}, fmt.Errorf("RDP connection port cannot be zero")
	}

	options, err := rdpoptions.Decode(connection.Options)
	if err != nil {
		return Invocation{}, fmt.Errorf("decode RDP options: %w", err)
	}

	client := c.client
	if client == nil {
		client = xfreerdpClient{}
	}
	return client.Plan(connection, options)
}

func (c RDPConnector) Connect(
	ctx context.Context,
	connection domain.Connection,
	streams IO,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	invocation, err := c.Plan(connection)
	if err != nil {
		return err
	}

	clientName := invocation.Program
	program, err := c.runner.LookPath(clientName)
	if err != nil {
		return fmt.Errorf(
			"%w: required client %q is not available: %w",
			ErrClientUnavailable,
			clientName,
			err,
		)
	}
	invocation.Program = program

	if err := c.runner.Run(ctx, invocation, streams); err != nil {
		return fmt.Errorf("run %s: %w", clientName, err)
	}
	return nil
}
