package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/Alurith/hoplane/internal/config"
	"github.com/Alurith/hoplane/internal/domain"
)

type pickerEditor struct {
	state *commandState
}

func newPickerEditor(state *commandState) *pickerEditor {
	return &pickerEditor{state: state}
}

func (e *pickerEditor) CanModify(connection domain.Connection) bool {
	return connection.Endpoint.Source.Name == "static"
}

func (e *pickerEditor) Create(ctx context.Context, candidate domain.Candidate) ([]domain.Connection, error) {
	path, err := e.state.path()
	if err != nil {
		return nil, err
	}
	connection, err := normalizeCandidate(candidate, path)
	if err != nil {
		return nil, err
	}
	if err := config.Update(ctx, path, func(file *config.File) error {
		if err := checkStaticNameAvailable(*file, connection.Name, -1); err != nil {
			return err
		}
		file.Connections = append(file.Connections, config.EntryFromConnection(connection))
		return nil
	}); err != nil {
		return nil, err
	}
	return e.reload(ctx)
}

func (e *pickerEditor) Update(
	ctx context.Context,
	original domain.Connection,
	candidate domain.Candidate,
) ([]domain.Connection, error) {
	if !e.CanModify(original) {
		return nil, fmt.Errorf("connection %q cannot be modified", original.Name)
	}
	path, err := e.state.path()
	if err != nil {
		return nil, err
	}
	connection, err := normalizeCandidate(candidate, path)
	if err != nil {
		return nil, err
	}
	if err := config.Update(ctx, path, func(file *config.File) error {
		index, ok := findStaticEntry(*file, original.Name)
		if !ok {
			return fmt.Errorf("static connection %q not found", original.Name)
		}
		if err := checkStaticNameAvailable(*file, connection.Name, index); err != nil {
			return err
		}
		file.Connections[index] = config.EntryFromConnection(connection)
		return nil
	}); err != nil {
		return nil, err
	}
	return e.reload(ctx)
}

func (e *pickerEditor) Duplicate(
	ctx context.Context,
	original domain.Connection,
	name string,
) ([]domain.Connection, error) {
	path, err := e.state.path()
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("name cannot be empty")
	}
	entry := config.EntryFromConnection(original)
	entry.Name = name
	connection, err := normalizeCandidate(entry.Candidate(domain.SourceRef{
		Name: "static",
		ID:   path,
	}), path)
	if err != nil {
		return nil, err
	}

	if err := config.Update(ctx, path, func(file *config.File) error {
		if err := checkStaticNameAvailable(*file, name, -1); err != nil {
			return err
		}
		file.Connections = append(file.Connections, config.EntryFromConnection(connection))
		return nil
	}); err != nil {
		return nil, err
	}
	return e.reload(ctx)
}

func (e *pickerEditor) Delete(ctx context.Context, connection domain.Connection) ([]domain.Connection, error) {
	if !e.CanModify(connection) {
		return nil, fmt.Errorf("connection %q cannot be deleted", connection.Name)
	}
	path, err := e.state.path()
	if err != nil {
		return nil, err
	}
	if err := config.Update(ctx, path, func(file *config.File) error {
		index, ok := findStaticEntry(*file, connection.Name)
		if !ok {
			return fmt.Errorf("static connection %q not found", connection.Name)
		}
		file.Connections = append(file.Connections[:index], file.Connections[index+1:]...)
		return nil
	}); err != nil {
		return nil, err
	}
	return e.reload(ctx)
}

func (e *pickerEditor) reload(ctx context.Context) ([]domain.Connection, error) {
	return e.state.loadCatalog(ctx)
}

func checkStaticNameAvailable(file config.File, name string, ignoredStaticIndex int) error {
	name = strings.TrimSpace(name)
	for index, entry := range file.Connections {
		if index == ignoredStaticIndex {
			continue
		}
		if strings.TrimSpace(entry.Name) == name {
			return fmt.Errorf("connection %q already exists", name)
		}
	}
	return nil
}

func findStaticEntry(file config.File, name string) (int, bool) {
	name = strings.TrimSpace(name)
	for index, entry := range file.Connections {
		if strings.TrimSpace(entry.Name) == name {
			return index, true
		}
	}
	return 0, false
}
