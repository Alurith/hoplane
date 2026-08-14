package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/Alurith/hoplane/internal/catalog"
	"github.com/Alurith/hoplane/internal/config"
	"github.com/Alurith/hoplane/internal/discovery"
	"github.com/Alurith/hoplane/internal/domain"
	"github.com/Alurith/hoplane/internal/output"
	"github.com/Alurith/hoplane/internal/tui"
	"github.com/spf13/cobra"
)

type Dependencies struct {
	Input  io.Reader
	Output io.Writer
	Errors io.Writer
}

type commandState struct {
	dependencies Dependencies
	configPath   string
}

func Execute(ctx context.Context, args []string, dependencies Dependencies) error {
	command := NewRootCommand(dependencies)
	command.SetArgs(args)
	command.SetContext(ctx)
	return command.Execute()
}

func NewRootCommand(dependencies Dependencies) *cobra.Command {
	state := &commandState{dependencies: dependencies}
	command := &cobra.Command{
		Use:           "hoplane",
		Short:         "A protocol-neutral connection directory",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE:          state.runPick,
	}
	command.PersistentFlags().StringVarP(&state.configPath, "config", "c", "", "path to the configuration file")
	command.AddCommand(
		state.newAddCommand(),
		state.newListCommand(),
		state.newShowCommand(),
		state.newPickCommand(),
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
	return catalog.Build(ctx, discovery.NewStaticSource(file, path))
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
	catalog, err := s.loadCatalog(ctx)
	if err != nil {
		return err
	}
	connection, selected, err := tui.Pick(ctx, catalog.Connections, s.dependencies.Input, s.dependencies.Output)
	if err != nil {
		return fmt.Errorf("run picker: %w", err)
	}
	if !selected {
		return nil
	}
	return output.WriteConnection(s.dependencies.Output, connection)
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
	return domain.NormalizeCandidate(entry.Candidate(domain.SourceRef{Name: "static", ID: sourcePath}))
}
