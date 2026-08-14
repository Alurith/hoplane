package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/Alurith/hoplane/internal/catalog"
	"github.com/Alurith/hoplane/internal/config"
	"github.com/Alurith/hoplane/internal/connector"
	"github.com/Alurith/hoplane/internal/discovery"
	"github.com/Alurith/hoplane/internal/domain"
	"github.com/Alurith/hoplane/internal/rdpoptions"
	"github.com/Alurith/hoplane/internal/sshoptions"
	"github.com/Alurith/hoplane/internal/tui"
	"github.com/spf13/cobra"
)

type Dependencies struct {
	Input  io.Reader
	Output io.Writer
	Errors io.Writer
}

type commandState struct {
	dependencies  Dependencies
	configPath    string
	sshConfigPath string
	registry      connector.Registry
}

func Execute(ctx context.Context, args []string, dependencies Dependencies) error {
	command := NewRootCommand(dependencies)
	command.SetArgs(args)
	command.SetContext(ctx)
	return command.Execute()
}

func NewRootCommand(dependencies Dependencies) *cobra.Command {
	state := &commandState{
		dependencies: dependencies,
		registry:     connector.DefaultRegistry(),
	}
	command := &cobra.Command{
		Use:           "hoplane",
		Short:         "A protocol-neutral connection directory",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE:          state.runPick,
	}
	command.SetIn(dependencies.Input)
	command.SetOut(dependencies.Output)
	command.SetErr(dependencies.Errors)
	command.PersistentFlags().StringVarP(&state.configPath, "config", "c", "", "path to the configuration file")
	command.PersistentFlags().StringVar(&state.sshConfigPath, "ssh-config", "", "path to the OpenSSH configuration file")
	command.AddCommand(
		state.newAddCommand(),
		state.newListCommand(),
		state.newShowCommand(),
		state.newPickCommand(),
		state.newConnectCommand(),
	)
	return command
}

func (s *commandState) loadCatalog(ctx context.Context) (catalog.Catalog, error) {
	path, err := s.path()
	if err != nil {
		return catalog.Catalog{}, err
	}
	file, err := config.Load(path)
	if err != nil {
		return catalog.Catalog{}, err
	}
	sshPath, err := s.sshPath()
	if err != nil {
		return catalog.Catalog{}, err
	}
	return catalog.Build(
		ctx,
		discovery.NewStaticSource(file, path),
		discovery.NewSSHConfigSource(sshPath),
	)
}

func (s *commandState) path() (string, error) {
	if s.configPath != "" {
		return s.configPath, nil
	}
	path, err := config.DefaultPath()
	if err != nil {
		return "", fmt.Errorf("resolve config path: %w", err)
	}
	return path, nil
}

func (s *commandState) sshPath() (string, error) {
	if s.sshConfigPath != "" {
		return s.sshConfigPath, nil
	}
	path, err := discovery.DefaultSSHConfigPath()
	if err != nil {
		return "", fmt.Errorf("resolve SSH config path: %w", err)
	}
	return path, nil
}

func (s *commandState) runPick(cmd *cobra.Command, _ []string) error {
	return s.pick(cmd.Context())
}

func (s *commandState) pick(ctx context.Context) error {
	catalog, err := s.loadCatalog(ctx)
	if err != nil {
		return err
	}
	editor := newPickerEditor(s)
	connection, action, err := tui.Pick(ctx, catalog.Connections, editor, s.dependencies.Input, s.dependencies.Output)
	if err != nil {
		return fmt.Errorf("run picker: %w", err)
	}
	if action == tui.ActionConnect {
		return s.connectConnection(ctx, connection, connector.ConnectOptions{})
	}
	return nil
}

func connectionNameExists(connections []config.Entry, name string) bool {
	name = strings.TrimSpace(name)
	for _, connection := range connections {
		if strings.TrimSpace(connection.Name) == name {
			return true
		}
	}
	return false
}

func normalizeEntry(entry config.Entry, sourcePath string) (domain.Connection, error) {
	return normalizeCandidate(entry.Candidate(domain.SourceRef{Name: "static", ID: sourcePath}), sourcePath)
}

func normalizeCandidate(candidate domain.Candidate, sourcePath string) (domain.Connection, error) {
	candidate.Source = domain.SourceRef{Name: "static", ID: sourcePath}
	connection, err := domain.NormalizeCandidate(candidate)
	if err != nil {
		return domain.Connection{}, err
	}
	if err := validateProtocolOptions(connection); err != nil {
		return domain.Connection{}, err
	}
	return connection, nil
}

func validateProtocolOptions(connection domain.Connection) error {
	switch connection.Endpoint.Protocol {
	case domain.ProtocolSSH:
		_, err := sshoptions.Decode(connection.Options)
		return err
	case domain.ProtocolRDP:
		_, err := rdpoptions.Decode(connection.Options)
		return err
	default:
		return nil
	}
}
