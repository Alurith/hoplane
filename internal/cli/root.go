package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/Alurith/hoplane/internal/catalog"
	"github.com/Alurith/hoplane/internal/config"
	"github.com/Alurith/hoplane/internal/connector"
	"github.com/Alurith/hoplane/internal/domain"
	"github.com/Alurith/hoplane/internal/secrets"
	"github.com/Alurith/hoplane/internal/selfupdate"
	"github.com/Alurith/hoplane/internal/tui"
	"github.com/spf13/cobra"
)

type Dependencies struct {
	Input       io.Reader
	Output      io.Writer
	Errors      io.Writer
	Version     string
	SecretStore secrets.Store
	Update      func(context.Context, string) (selfupdate.Result, error)
}

type commandState struct {
	dependencies Dependencies
	configPath   string
	version      string
	registry     connector.Registry
	secretStore  secrets.Store
}

func Execute(ctx context.Context, args []string, dependencies Dependencies) error {
	command := NewRootCommand(dependencies)
	command.SetArgs(args)
	command.SetContext(ctx)
	return command.Execute()
}

func NewRootCommand(dependencies Dependencies) *cobra.Command {
	version := dependencies.Version
	if version == "" {
		version = selfupdate.DevelopmentVersion
	}
	secretStore := dependencies.SecretStore
	if secretStore == nil {
		secretStore = secrets.NewDefaultStore()
	}
	state := &commandState{
		dependencies: dependencies,
		version:      version,
		registry:     connector.DefaultRegistry(secretStore),
		secretStore:  secretStore,
	}
	command := &cobra.Command{
		Use:           "hoplane",
		Short:         "A protocol-neutral connection directory",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE:          state.runPick,
	}
	command.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	command.SetIn(dependencies.Input)
	command.SetOut(dependencies.Output)
	command.SetErr(dependencies.Errors)
	command.PersistentFlags().StringVarP(&state.configPath, "config", "c", "", "path to the configuration file")
	command.AddCommand(
		state.newAddCommand(),
		state.newListCommand(),
		state.newShowCommand(),
		state.newPickCommand(),
		state.newConnectCommand(),
		state.newSecretCommand(),
		state.newUpdateCommand(),
	)
	return command
}

func (s *commandState) loadCatalog(ctx context.Context) ([]domain.Connection, error) {
	path, err := s.path()
	if err != nil {
		return nil, err
	}
	file, err := config.Load(path)
	if err != nil {
		return nil, err
	}
	return catalog.Build(ctx, file, path)
}

func (s *commandState) findConnection(ctx context.Context, name string) (domain.Connection, error) {
	connections, err := s.loadCatalog(ctx)
	if err != nil {
		return domain.Connection{}, err
	}
	for _, connection := range connections {
		if connection.Name == name {
			return connection, nil
		}
	}
	return domain.Connection{}, fmt.Errorf("connection %q not found", name)
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

func (s *commandState) runPick(cmd *cobra.Command, _ []string) error {
	return s.pick(cmd.Context())
}

func (s *commandState) pick(ctx context.Context) error {
	connections, err := s.loadCatalog(ctx)
	if err != nil {
		return err
	}
	editor := newPickerEditor(s)
	connection, action, err := tui.Pick(ctx, connections, editor, s.dependencies.Input, s.dependencies.Output)
	if err != nil {
		return fmt.Errorf("run picker: %w", err)
	}
	if action == tui.ActionConnect {
		return s.connectConnection(ctx, connection, false)
	}
	return nil
}

func normalizeCandidate(candidate domain.Candidate, sourcePath string) (domain.Connection, error) {
	if strings.TrimSpace(candidate.ID) == "" {
		id, err := domain.NewConnectionID()
		if err != nil {
			return domain.Connection{}, err
		}
		candidate.ID = id
	}
	candidate.Source = domain.SourceRef{Name: "static", ID: sourcePath}
	return domain.NormalizeCandidate(candidate)
}
