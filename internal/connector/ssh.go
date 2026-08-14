package connector

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Alurith/hoplane/internal/domain"
)

// SSHConnector is the minimal process-backed SSH connector. SSH-specific
// configuration remains in the connector rather than in the common domain.
type SSHConnector struct {
	runner ProcessRunner
}

func NewSSHConnector(runner ProcessRunner) SSHConnector {
	if runner == nil {
		runner = ExecRunner{}
	}
	return SSHConnector{runner: runner}
}

func (SSHConnector) Protocol() domain.Protocol {
	return domain.ProtocolSSH
}

func (c SSHConnector) Plan(connection domain.Connection) (Invocation, error) {
	if connection.Endpoint.Protocol != c.Protocol() {
		return Invocation{}, fmt.Errorf(
			"SSH connector cannot handle protocol %q",
			connection.Endpoint.Protocol,
		)
	}
	if connection.Endpoint.Host == "" {
		return Invocation{}, fmt.Errorf("SSH connection host cannot be empty")
	}
	if connection.Endpoint.Port == 0 {
		return Invocation{}, fmt.Errorf("SSH connection port cannot be zero")
	}

	host := connection.Endpoint.Host
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	target := host
	if connection.Endpoint.User != "" {
		target = connection.Endpoint.User + "@" + host
	}

	return Invocation{
		Program: "ssh",
		Args: []string{
			"-p",
			strconv.FormatUint(uint64(connection.Endpoint.Port), 10),
			target,
		},
	}, nil
}

func (c SSHConnector) Connect(
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

	program, err := c.runner.LookPath(invocation.Program)
	if err != nil {
		return fmt.Errorf("required client %q is not available: %w", invocation.Program, err)
	}
	invocation.Program = program

	if err := c.runner.Run(ctx, invocation, streams); err != nil {
		return fmt.Errorf("run ssh: %w", err)
	}
	return nil
}
