package discovery

import (
	"context"

	"github.com/Alurith/hoplane/internal/config"
	"github.com/Alurith/hoplane/internal/domain"
)

// StaticSource exposes the declarative configuration as a discovery source.
type StaticSource struct {
	file config.File
	path string
}

func NewStaticSource(file config.File, path string) StaticSource {
	return StaticSource{file: file, path: path}
}

func (s StaticSource) Name() string { return "static" }

func (s StaticSource) Discover(ctx context.Context) ([]domain.Candidate, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	candidates := make([]domain.Candidate, 0, len(s.file.Connections))
	source := domain.SourceRef{Name: s.Name(), ID: s.path}
	for _, entry := range s.file.Connections {
		candidates = append(candidates, entry.Candidate(source))
	}
	return candidates, nil
}
