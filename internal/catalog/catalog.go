package catalog

import (
	"context"
	"fmt"
	"sort"

	"github.com/Alurith/hoplane/internal/discovery"
	"github.com/Alurith/hoplane/internal/domain"
)

// Warning describes a non-fatal discovery failure.
type Warning struct {
	Source  string
	Message string
}

// Catalog is the normalized view consumed by CLI output and the picker.
type Catalog struct {
	Connections []domain.Connection
	Warnings    []Warning
}

func Build(ctx context.Context, sources ...discovery.Source) (Catalog, error) {
	return build(ctx, false, sources...)
}

// BuildWithWarnings keeps the catalog usable when an optional source fails.
// Context cancellation by the caller remains fatal.
func BuildWithWarnings(ctx context.Context, sources ...discovery.Source) (Catalog, error) {
	return build(ctx, true, sources...)
}

func build(ctx context.Context, tolerateSourceErrors bool, sources ...discovery.Source) (Catalog, error) {
	connections := make([]domain.Connection, 0)
	warnings := make([]Warning, 0)
	seenNames := make(map[string]struct{})

	for _, source := range sources {
		candidates, err := source.Discover(ctx)
		if err != nil {
			if ctx.Err() != nil || !tolerateSourceErrors {
				return Catalog{}, fmt.Errorf("discover from %s: %w", source.Name(), err)
			}
			warnings = append(warnings, Warning{Source: source.Name(), Message: err.Error()})
			continue
		}
		for _, candidate := range candidates {
			if err := ctx.Err(); err != nil {
				return Catalog{}, err
			}
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
	return Catalog{Connections: connections, Warnings: warnings}, nil
}

// Merge combines already-normalized catalogs and preserves source warnings.
func Merge(catalogs ...Catalog) (Catalog, error) {
	connections := make([]domain.Connection, 0)
	warnings := make([]Warning, 0)
	seenNames := make(map[string]struct{})
	for _, value := range catalogs {
		warnings = append(warnings, value.Warnings...)
		for _, connection := range value.Connections {
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
	return Catalog{Connections: connections, Warnings: warnings}, nil
}

func (c Catalog) Find(name string) (domain.Connection, bool) {
	for _, connection := range c.Connections {
		if connection.Name == name {
			return connection, true
		}
	}
	return domain.Connection{}, false
}
