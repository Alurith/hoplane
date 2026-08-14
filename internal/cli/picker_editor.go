package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/Alurith/hoplane/internal/catalog"
	"github.com/Alurith/hoplane/internal/config"
	"github.com/Alurith/hoplane/internal/discovery"
	"github.com/Alurith/hoplane/internal/domain"
	"github.com/Alurith/hoplane/internal/rdpoptions"
	"github.com/Alurith/hoplane/internal/sshoptions"
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
	file, err := config.Load(path)
	if err != nil {
		return nil, err
	}
	connection, err := normalizePickerCandidate(candidate, path)
	if err != nil {
		return nil, err
	}
	if err := e.checkNameAvailable(ctx, file, connection.Name, -1); err != nil {
		return nil, err
	}

	file.Connections = append(file.Connections, config.EntryFromConnection(connection))
	if err := config.Save(path, file); err != nil {
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
	file, err := config.Load(path)
	if err != nil {
		return nil, err
	}
	index, ok := findStaticEntry(file, original.Name)
	if !ok {
		return nil, fmt.Errorf("static connection %q not found", original.Name)
	}
	connection, err := normalizePickerCandidate(candidate, path)
	if err != nil {
		return nil, err
	}
	if err := e.checkNameAvailable(ctx, file, connection.Name, index); err != nil {
		return nil, err
	}

	file.Connections[index] = config.EntryFromConnection(connection)
	if err := config.Save(path, file); err != nil {
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
	file, err := config.Load(path)
	if err != nil {
		return nil, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("name cannot be empty")
	}
	if err := e.checkNameAvailable(ctx, file, name, -1); err != nil {
		return nil, err
	}

	entry := config.EntryFromConnection(original)
	entry.Name = name
	connection, err := normalizePickerCandidate(entry.Candidate(domain.SourceRef{
		Name: "static",
		ID:   path,
	}), path)
	if err != nil {
		return nil, err
	}
	file.Connections = append(file.Connections, config.EntryFromConnection(connection))
	if err := config.Save(path, file); err != nil {
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
	file, err := config.Load(path)
	if err != nil {
		return nil, err
	}
	index, ok := findStaticEntry(file, connection.Name)
	if !ok {
		return nil, fmt.Errorf("static connection %q not found", connection.Name)
	}
	file.Connections = append(file.Connections[:index], file.Connections[index+1:]...)
	if err := config.Save(path, file); err != nil {
		return nil, err
	}
	return e.reload(ctx)
}

func (e *pickerEditor) reload(ctx context.Context) ([]domain.Connection, error) {
	catalog, err := e.state.loadCatalog(ctx)
	if err != nil {
		return nil, err
	}
	return catalog.Connections, nil
}

func (e *pickerEditor) checkNameAvailable(
	ctx context.Context,
	file config.File,
	name string,
	ignoredStaticIndex int,
) error {
	name = strings.TrimSpace(name)
	for index, entry := range file.Connections {
		if index == ignoredStaticIndex {
			continue
		}
		if strings.TrimSpace(entry.Name) == name {
			return fmt.Errorf("connection %q already exists", name)
		}
	}

	sshPath, err := e.state.sshPath()
	if err != nil {
		return err
	}
	sshCatalog, err := catalog.Build(ctx, discovery.NewSSHConfigSource(sshPath))
	if err != nil {
		return err
	}
	if _, exists := sshCatalog.Find(name); exists {
		return fmt.Errorf("connection %q already exists", name)
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

func normalizePickerCandidate(candidate domain.Candidate, path string) (domain.Connection, error) {
	candidate.Source = domain.SourceRef{Name: "static", ID: path}
	connection, err := domain.NormalizeCandidate(candidate)
	if err != nil {
		return domain.Connection{}, err
	}
	switch connection.Endpoint.Protocol {
	case domain.ProtocolSSH:
		if _, err := sshoptions.Decode(connection.Options); err != nil {
			return domain.Connection{}, err
		}
	case domain.ProtocolRDP:
		if _, err := rdpoptions.Decode(connection.Options); err != nil {
			return domain.Connection{}, err
		}
	}
	return connection, nil
}
