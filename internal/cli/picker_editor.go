package cli

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/Alurith/hoplane/internal/catalog"
	"github.com/Alurith/hoplane/internal/config"
	"github.com/Alurith/hoplane/internal/domain"
	"github.com/Alurith/hoplane/internal/secrets"
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
	candidate.ID = ""
	var connections []domain.Connection
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
		connections, err = catalog.Build(ctx, *file, path)
		return err
	}); err != nil {
		return nil, err
	}
	return connections, nil
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
	candidate.ID = original.ID
	var connections []domain.Connection
	connection, err := normalizeCandidate(candidate, path)
	if err != nil {
		return nil, err
	}
	if err := config.WithLock(ctx, path, func() error {
		file, err := config.Load(path)
		if err != nil {
			return err
		}
		index, ok := findStaticEntry(file, original.ID)
		if !ok {
			return fmt.Errorf("static connection %q not found", original.Name)
		}
		current, err := normalizeCandidate(file.Connections[index].Candidate(original.Endpoint.Source), path)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(current, original) {
			return fmt.Errorf("connection %q changed; reload before editing", original.Name)
		}
		if err := checkStaticNameAvailable(file, connection.Name, index); err != nil {
			return err
		}
		file.Connections[index] = config.EntryFromConnection(connection)
		connections, err = catalog.Build(ctx, file, path)
		if err != nil {
			return err
		}
		save := func() error { return config.Save(path, file) }
		if current.Endpoint.Protocol != connection.Endpoint.Protocol {
			return e.state.deletePasswordsAndSave(ctx, current, []string{string(current.Endpoint.Protocol)}, save)
		}
		return save()
	}); err != nil {
		return nil, err
	}
	return connections, nil
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
	entry.ID = ""
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
	if err := config.WithLock(ctx, path, func() error {
		file, err := config.Load(path)
		if err != nil {
			return err
		}
		index, ok := findStaticEntry(file, connection.ID)
		if !ok {
			return fmt.Errorf("static connection %q not found", connection.Name)
		}
		file.Connections = append(file.Connections[:index], file.Connections[index+1:]...)
		return e.state.deletePasswordsAndSave(ctx, connection, []string{secrets.ProtocolSSH, secrets.ProtocolRDP}, func() error {
			return config.Save(path, file)
		})
	}); err != nil {
		return nil, err
	}
	return e.reload(ctx)
}

func (e *pickerEditor) reload(ctx context.Context) ([]domain.Connection, error) {
	return e.state.loadCatalog(ctx)
}

func (e *pickerEditor) SetPassword(ctx context.Context, connection domain.Connection, value []byte) error {
	if err := secrets.ValidateSecretForProtocol(string(connection.Endpoint.Protocol), value); err != nil {
		return err
	}
	return e.state.storePassword(ctx, connection, value)
}

func (e *pickerEditor) ClearPassword(ctx context.Context, connection domain.Connection) error {
	return e.state.clearPassword(ctx, connection)
}

func passwordRef(connection domain.Connection) secrets.Ref {
	protocol := string(connection.Endpoint.Protocol)
	return secrets.Ref{
		ConnectionID: connection.ID,
		Protocol:     protocol,
		Kind:         secrets.LoginPassword,
	}
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

func findStaticEntry(file config.File, id string) (int, bool) {
	id = strings.TrimSpace(id)
	for index, entry := range file.Connections {
		if strings.TrimSpace(entry.ID) == id {
			return index, true
		}
	}
	return 0, false
}
