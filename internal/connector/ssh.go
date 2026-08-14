package connector

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"github.com/Alurith/hoplane/internal/domain"
	"github.com/Alurith/hoplane/internal/sshoptions"
)

// SSHConnector is a process-backed SSH connector. SSH-specific configuration
// remains in this adapter rather than in the common domain model.
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

	options, err := sshoptions.Decode(connection.Options)
	if err != nil {
		return Invocation{}, fmt.Errorf("decode SSH options: %w", err)
	}

	args := make([]string, 0, 10)
	if options.ConfigFile != "" {
		path, err := expandUserPath(options.ConfigFile)
		if err != nil {
			return Invocation{}, fmt.Errorf("resolve SSH config path: %w", err)
		}
		args = append(args, "-F", path)
	}
	if options.IdentityFile != "" {
		path, err := expandUserPath(options.IdentityFile)
		if err != nil {
			return Invocation{}, fmt.Errorf("resolve SSH identity file: %w", err)
		}
		args = append(args, "-i", path)
	}
	if options.ProxyJump != "" {
		args = append(args, "-J", options.ProxyJump)
	}

	if options.HostAlias != "" {
		// Do not override the alias's User or Port. OpenSSH must apply the
		// complete configuration for the alias, including HostName and Match.
		args = append(args, options.HostAlias)
	} else {
		args = append(args, "-p", strconv.FormatUint(uint64(connection.Endpoint.Port), 10))
		host := connection.Endpoint.Host
		if strings.Contains(host, ":") {
			host = "[" + host + "]"
		}
		target := host
		if connection.Endpoint.User != "" {
			target = connection.Endpoint.User + "@" + host
		}
		args = append(args, target)
	}

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

	if err := c.runner.Run(ctx, invocation, streams); err != nil {
		return fmt.Errorf("run ssh: %w", err)
	}
	return nil
}

func expandUserPath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("path cannot be empty")
	}
	if value != "~" && !strings.HasPrefix(value, "~/") && !strings.HasPrefix(value, `~\`) {
		return filepath.Clean(value), nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if value == "~" {
		return filepath.Clean(home), nil
	}
	suffix := strings.TrimLeft(value[1:], `/\`)
	for _, r := range suffix {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("path contains a control character")
		}
	}
	return filepath.Clean(filepath.Join(home, filepath.FromSlash(suffix))), nil
}
