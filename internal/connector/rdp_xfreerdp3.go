//go:build linux

package connector

import (
	"net"
	"strconv"

	"github.com/Alurith/hoplane/internal/domain"
	"github.com/Alurith/hoplane/internal/rdpoptions"
)

const rdpProgram = "xfreerdp3"

func platformRDPPlan(
	connection domain.Connection,
	options rdpoptions.Options,
) (string, []string, error) {
	address := net.JoinHostPort(
		connection.Endpoint.Host,
		strconv.FormatUint(uint64(connection.Endpoint.Port), 10),
	)
	args := []string{"/v:" + address}
	if connection.Endpoint.User != "" {
		args = append(args, "/u:"+connection.Endpoint.User)
	}
	if options.Domain != "" {
		args = append(args, "/d:"+options.Domain)
	}
	if options.Fullscreen {
		args = append(args, "/f")
	}
	if options.IgnoreCertificate {
		args = append(args, "/cert:ignore")
	}
	return rdpProgram, args, nil
}
