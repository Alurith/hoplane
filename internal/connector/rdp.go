package connector

import (
	"context"
	"fmt"
	"runtime"

	"github.com/Alurith/hoplane/internal/domain"
	"github.com/Alurith/hoplane/internal/rdpoptions"
)

// RDPConnector selects a registered platform client and executes its plan.
type RDPConnector struct {
	runner        ProcessRunner
	defaultClient string
	clients       map[string]rdpClient
}

type rdpClient interface {
	ID() string
	Program() string
	Capabilities() rdpClientCapabilities
	Plan(domain.Connection, rdpoptions.Options) ([]string, error)
}

type rdpClientCapabilities struct {
	Fullscreen        bool
	IgnoreCertificate bool
}

func NewRDPConnector(runner ProcessRunner) RDPConnector {
	defaultClient, clients := platformRDPClients()
	return newRDPConnector(runner, defaultClient, clients)
}

func newRDPConnector(
	runner ProcessRunner,
	defaultClient string,
	clients map[string]rdpClient,
) RDPConnector {
	if runner == nil {
		runner = ExecRunner{}
	}
	return RDPConnector{
		runner:        runner,
		defaultClient: defaultClient,
		clients:       clients,
	}
}

func (RDPConnector) Protocol() domain.Protocol {
	return domain.ProtocolRDP
}

func (c RDPConnector) Plan(connection domain.Connection) (Invocation, error) {
	_, invocation, err := c.plan(connection)
	return invocation, err
}

func (c RDPConnector) plan(connection domain.Connection) (string, Invocation, error) {
	if connection.Endpoint.Protocol != c.Protocol() {
		return "", Invocation{}, fmt.Errorf(
			"RDP connector cannot handle protocol %q",
			connection.Endpoint.Protocol,
		)
	}
	if connection.Endpoint.Host == "" {
		return "", Invocation{}, fmt.Errorf("RDP connection host cannot be empty")
	}
	if connection.Endpoint.Port == 0 {
		return "", Invocation{}, fmt.Errorf("RDP connection port cannot be zero")
	}

	options, err := rdpoptions.Decode(connection.Options)
	if err != nil {
		return "", Invocation{}, fmt.Errorf("decode RDP options: %w", err)
	}

	clientID := options.Client
	if clientID == "" {
		clientID = c.defaultClient
	}
	if clientID == "" {
		return "", Invocation{}, fmt.Errorf("no RDP client is registered for platform %q", runtime.GOOS)
	}
	client, ok := c.clients[clientID]
	if !ok || client == nil {
		return "", Invocation{}, fmt.Errorf(
			"RDP client %q is not registered for platform %q",
			clientID,
			runtime.GOOS,
		)
	}
	if client.ID() != clientID {
		return "", Invocation{}, fmt.Errorf(
			"RDP client registration %q has adapter ID %q",
			clientID,
			client.ID(),
		)
	}
	if err := validateRDPClientOptions(clientID, client.Capabilities(), options); err != nil {
		return "", Invocation{}, err
	}
	program := client.Program()
	if err := validateExecutableName(program); err != nil {
		return "", Invocation{}, fmt.Errorf("RDP client %q has invalid executable: %w", clientID, err)
	}
	args, err := client.Plan(connection, options)
	if err != nil {
		return "", Invocation{}, fmt.Errorf("plan with RDP client %q: %w", clientID, err)
	}
	return clientID, Invocation{Program: program, Args: args}, nil
}

func validateRDPClientOptions(
	clientID string,
	capabilities rdpClientCapabilities,
	options rdpoptions.Options,
) error {
	if options.Fullscreen && !capabilities.Fullscreen {
		return fmt.Errorf("RDP client %q does not support option %q", clientID, rdpoptions.Fullscreen)
	}
	if options.IgnoreCertificate && !capabilities.IgnoreCertificate {
		return fmt.Errorf("RDP client %q does not support option %q", clientID, rdpoptions.IgnoreCertificate)
	}
	return nil
}

func (c RDPConnector) Connect(
	ctx context.Context,
	connection domain.Connection,
	streams IO,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	clientID, invocation, err := c.plan(connection)
	if err != nil {
		return err
	}

	programName := invocation.Program
	program, err := c.runner.LookPath(programName)
	if err != nil {
		return fmt.Errorf(
			"%w: executable %q for RDP client %q is not available: %w",
			ErrClientUnavailable,
			programName,
			clientID,
			err,
		)
	}
	invocation.Program = program

	if err := c.runner.Run(ctx, invocation, streams); err != nil {
		return fmt.Errorf("run %s: %w", programName, err)
	}
	return nil
}
