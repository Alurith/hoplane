package discovery

import (
	"context"

	"github.com/Alurith/hoplane/internal/domain"
)

// Source discovers raw candidates. Sources do not normalize or merge results.
type Source interface {
	Name() string
	Discover(context.Context) ([]domain.Candidate, error)
}
