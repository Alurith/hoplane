package tui

import (
	"context"

	"github.com/Alurith/hoplane/internal/domain"
)

// ConnectionEditor supplies the picker with persistence operations without
// coupling the TUI to the configuration format or filesystem.
type ConnectionEditor interface {
	CanModify(domain.Connection) bool

	Create(context.Context, domain.Candidate) ([]domain.Connection, error)
	Update(context.Context, domain.Connection, domain.Candidate) ([]domain.Connection, error)
	Duplicate(context.Context, domain.Connection, string) ([]domain.Connection, error)
	Delete(context.Context, domain.Connection) ([]domain.Connection, error)
}

// SecretEditor is implemented by editors that manage the connection password
// outside the catalog. The password never crosses the domain model.
type SecretEditor interface {
	SetPassword(context.Context, domain.Connection, []byte) error
	ClearPassword(context.Context, domain.Connection) error
}
