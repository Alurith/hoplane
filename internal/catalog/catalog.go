package catalog

import (
	"context"
	"fmt"
	"sort"

	"github.com/Alurith/hoplane/internal/config"
	"github.com/Alurith/hoplane/internal/domain"
)

// Build normalizes the static configuration into the sorted connection list
// consumed by the CLI and picker.
func Build(ctx context.Context, file config.File, path string) ([]domain.Connection, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	connections := make([]domain.Connection, 0, len(file.Connections))
	seenNames := make(map[string]struct{}, len(file.Connections))
	source := domain.SourceRef{Name: "static", ID: path}
	for _, entry := range file.Connections {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		candidate := entry.Candidate(source)
		connection, err := domain.NormalizeCandidate(candidate)
		if err != nil {
			return nil, fmt.Errorf("normalize static candidate %q: %w", candidate.Name, err)
		}
		if _, exists := seenNames[connection.Name]; exists {
			return nil, fmt.Errorf("duplicate connection name %q", connection.Name)
		}
		seenNames[connection.Name] = struct{}{}
		connections = append(connections, connection)
	}

	sort.SliceStable(connections, func(i, j int) bool {
		return connections[i].Name < connections[j].Name
	})
	return connections, nil
}
