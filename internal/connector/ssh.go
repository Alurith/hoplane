package connector

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/Alurith/hoplane/internal/domain"
	"github.com/Alurith/hoplane/internal/secrets"
)

// SSHConnector is a process-backed SSH connector for standard endpoints.
type SSHConnector struct {
	runner      ProcessRunner
	secretStore secrets.Store
}

func NewSSHConnector(runner ProcessRunner, store ...secrets.Store) SSHConnector {
	if runner == nil {
		runner = ExecRunner{}
	}
	var secretStore secrets.Store
	if len(store) > 0 {
		secretStore = store[0]
	}
	if secretStore == nil {
		secretStore = secrets.NewDefaultStore()
	}
	return SSHConnector{runner: runner, secretStore: secretStore}
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
	if len(connection.Options) > 0 {
		return Invocation{}, fmt.Errorf("SSH options are not supported")
	}

	args := []string{"-p", strconv.FormatUint(uint64(connection.Endpoint.Port), 10)}
	if connection.Endpoint.User != "" {
		args = append(args, "-l", connection.Endpoint.User)
	}
	args = append(args, "--", connection.Endpoint.Host)
	return Invocation{Program: "ssh", Args: args}, nil
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
		return fmt.Errorf(
			"%w: required client %q is not available: %w",
			ErrClientUnavailable,
			invocation.Program,
			err,
		)
	}
	invocation.Program = program

	secret, err := c.lookupPassword(ctx, connection)
	if errors.Is(err, secrets.ErrNotFound) {
		secrets.Clear(secret)
		if err := c.runner.Run(ctx, invocation, streams); err != nil {
			return fmt.Errorf("run ssh: %w", err)
		}
		return nil
	}
	if err != nil {
		secrets.Clear(secret)
		return err
	}
	if err := secrets.ValidateSecretForProtocol(secrets.ProtocolSSH, secret); err != nil {
		secrets.Clear(secret)
		return err
	}

	secureRunner, ok := c.runner.(SecretProcessRunner)
	if !ok {
		secrets.Clear(secret)
		return fmt.Errorf("%w for SSH", ErrSecureTransport)
	}
	invocation.Args = sshPasswordArgs(invocation.Args)
	if err := secureRunner.RunWithSecret(ctx, invocation, streams, SecretProcessRequest{
		Transport: SecretTransportSSHAskpass,
		Payload:   secret,
	}); err != nil {
		return fmt.Errorf("run ssh with keyring password: %w", err)
	}
	return nil
}

func (c SSHConnector) lookupPassword(ctx context.Context, connection domain.Connection) ([]byte, error) {
	if connection.ID == "" || c.secretStore == nil {
		return nil, secrets.ErrNotFound
	}
	return c.secretStore.Lookup(ctx, secrets.Ref{
		ConnectionID: connection.ID,
		Protocol:     secrets.ProtocolSSH,
		Kind:         secrets.LoginPassword,
	})
}

func sshPasswordArgs(args []string) []string {
	separator := len(args)
	for index, value := range args {
		if value == "--" {
			separator = index
			break
		}
	}
	options := []string{
		"-o", "PreferredAuthentications=password",
		"-o", "KbdInteractiveAuthentication=no",
		"-o", "NumberOfPasswordPrompts=1",
	}
	result := make([]string, 0, len(args)+len(options))
	result = append(result, args[:separator]...)
	result = append(result, options...)
	result = append(result, args[separator:]...)
	return result
}
