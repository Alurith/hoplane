package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"reflect"

	"github.com/Alurith/hoplane/internal/catalog"
	"github.com/Alurith/hoplane/internal/config"
	"github.com/Alurith/hoplane/internal/domain"
	"github.com/Alurith/hoplane/internal/secrets"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func (s *commandState) newSecretCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "secret",
		Short: "Manage connection passwords in the system keyring",
	}
	command.AddCommand(
		s.newSecretSetCommand(),
		s.newSecretClearCommand(),
		s.newSecretStatusCommand(),
	)
	return command
}

func (s *commandState) newSecretSetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set <connection>",
		Short: "Store a connection password without exposing it in the command line",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			connection, err := s.findConnection(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			value, err := readSecret(s.dependencies.Input, cmd.OutOrStdout(), string(connection.Endpoint.Protocol))
			if err != nil {
				return err
			}
			defer secrets.Clear(value)
			if err := s.storePassword(cmd.Context(), connection, value); err != nil {
				return fmt.Errorf("store password: %w", err)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "stored password for %q\n", connection.Name)
			return err
		},
	}
}

func (s *commandState) newSecretClearCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "clear <connection>",
		Short: "Remove a connection password from the system keyring",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			connection, err := s.findConnection(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if err := s.clearPassword(cmd.Context(), connection); err != nil {
				return fmt.Errorf("clear password: %w", err)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "cleared password for %q\n", connection.Name)
			return err
		},
	}
}

func (s *commandState) newSecretStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status <connection>",
		Short: "Show whether a connection has a stored password",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			connection, err := s.findConnection(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			present, err := s.secretStore.Has(cmd.Context(), passwordRef(connection))
			if err != nil {
				return fmt.Errorf("check password: %w", err)
			}
			status := "not configured"
			if present {
				status = "configured"
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", connection.Name, status)
			return err
		},
	}
}

func (s *commandState) storePassword(ctx context.Context, connection domain.Connection, value []byte) error {
	path, err := s.path()
	if err != nil {
		return err
	}
	return config.WithLock(ctx, path, func() error {
		file, err := config.Load(path)
		if err != nil {
			return err
		}
		connections, err := catalog.Build(ctx, file, path)
		if err != nil {
			return err
		}
		for _, current := range connections {
			if current.ID == connection.ID {
				if current.Endpoint != connection.Endpoint || !reflect.DeepEqual(current.Options, connection.Options) {
					return fmt.Errorf("connection %q target changed; reload before setting its password", connection.Name)
				}
				return s.secretStore.Store(ctx, passwordRef(current), value)
			}
		}
		return fmt.Errorf("connection %q no longer exists", connection.Name)
	})
}

func (s *commandState) clearPassword(ctx context.Context, connection domain.Connection) error {
	path, err := s.path()
	if err != nil {
		return err
	}
	return config.WithLock(ctx, path, func() error {
		return s.clearPasswordLocked(ctx, connection)
	})
}

func (s *commandState) clearPasswordLocked(ctx context.Context, connection domain.Connection) error {
	var firstErr error
	for _, protocol := range []string{secrets.ProtocolSSH, secrets.ProtocolRDP} {
		if err := s.clearPasswordProtocolLocked(ctx, connection, protocol); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (s *commandState) clearPasswordProtocolLocked(ctx context.Context, connection domain.Connection, protocol string) error {
	ref := passwordRef(connection)
	ref.Protocol = protocol
	return s.secretStore.Delete(ctx, ref)
}

func readSecret(input io.Reader, output io.Writer, protocol string) ([]byte, error) {
	max, err := secrets.MaxSecretBytesForProtocol(protocol)
	if err != nil {
		return nil, err
	}
	if file, ok := input.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		if _, err := fmt.Fprint(output, "Password: "); err != nil {
			return nil, err
		}
		value, err := term.ReadPassword(int(file.Fd()))
		_, _ = fmt.Fprintln(output)
		if err != nil {
			secrets.Clear(value)
			return nil, fmt.Errorf("read password: %w", err)
		}
		if err := secrets.ValidateSecretForProtocol(protocol, value); err != nil {
			secrets.Clear(value)
			return nil, err
		}
		return value, nil
	}

	raw, err := io.ReadAll(io.LimitReader(input, int64(max)+1))
	if err != nil {
		secrets.Clear(raw)
		return nil, fmt.Errorf("read password: %w", err)
	}
	value := raw
	if bytes.HasSuffix(value, []byte("\n")) {
		value = value[:len(value)-1]
		if bytes.HasSuffix(value, []byte("\r")) {
			value = value[:len(value)-1]
		}
	} else if len(value) == max+1 && bytes.HasSuffix(value, []byte("\r")) {
		// A max-sized password followed by CRLF fills the max+1 sentinel
		// before the LF can be read. Consume that one explicit terminator byte.
		var terminator [1]byte
		n, _ := input.Read(terminator[:])
		if n == 1 && terminator[0] == '\n' {
			value = value[:len(value)-1]
		}
	}
	value = append([]byte(nil), value...)
	secrets.Clear(raw)
	if err := secrets.ValidateSecretForProtocol(protocol, value); err != nil {
		secrets.Clear(value)
		return nil, err
	}
	return value, nil
}
