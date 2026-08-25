//go:build !linux

package connector

import (
	"fmt"
	"runtime"

	"github.com/Alurith/hoplane/internal/domain"
	"github.com/Alurith/hoplane/internal/rdpoptions"
)

func platformRDPPlan(domain.Connection, rdpoptions.Options) (string, []string, error) {
	return "", nil, fmt.Errorf("RDP is unavailable on platform %q", runtime.GOOS)
}
