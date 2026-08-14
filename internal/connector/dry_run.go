package connector

import (
	"fmt"
	"io"

	"github.com/Alurith/hoplane/internal/domain"
)

func WriteDryRun(w io.Writer, connection domain.Connection, invocation Invocation) error {
	if _, err := fmt.Fprintf(
		w,
		"dry-run: connection %q would execute %s\n",
		connection.Name,
		invocation,
	); err != nil {
		return fmt.Errorf("write dry-run: %w", err)
	}
	return nil
}
