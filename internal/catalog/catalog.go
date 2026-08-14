package catalog

import (
	"context"
	"fmt"
	"sort"

	"github.com/Alurith/hoplane/internal/discovery"
	"github.com/Alurith/hoplane/internal/domain"
)

// Catalog is the normalized view consumed by CLI output and the picker.
type Catalog struct {
	Connections []domain.Connection
}

func Build(ctx context.Context, sources ...discovery.Source) (Catalog, error) {
	connections := make([]domain.Connection, 0)
	seenNames := make(map[string]struct{})

	for _, source := range sources {
		candidates, err := source.Discover(ctx)
		if err != nil {
			return Catalog{}, fmt.Errorf("discover from %s: %w", source.Name(), err)
		}
		for _, candidate := range candidates {
			connection, err := domain.NormalizeCandidate(candidate)
			if err != nil {
				return Catalog{}, fmt.Errorf("normalize %s candidate %q: %w", source.Name(), candidate.Name, err)
			}
			if _, exists := seenNames[connection.Name]; exists {
				return Catalog{}, fmt.Errorf("duplicate connection name %q", connection.Name)
			}
			seenNames[connection.Name] = struct{}{}
			connections = append(connections, connection)
		}
	}

	sort.SliceStable(connections, func(i, j int) bool {
		return connections[i].Name < connections[j].Name
	})
	return Catalog{Connections: connections}, nil
}

func (c Catalog) Find(name string) (domain.Connection, bool) {
	for _, connection := range c.Connections {
		if connection.Name == name {
			return connection, true
		}
	}
	return domain.Connection{}, false
}
