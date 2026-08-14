package connector

import (
	"fmt"
	"net"
	"strconv"

	"github.com/Alurith/hoplane/internal/domain"
	"github.com/Alurith/hoplane/internal/rdpoptions"
)

type xfreerdpClient struct{}

func (xfreerdpClient) Name() string {
	return "xfreerdp"
}

func (c xfreerdpClient) Plan(
	connection domain.Connection,
	options rdpoptions.Options,
) (Invocation, error) {
	if options.Client != "" && options.Client != c.Name() {
		return Invocation{}, fmt.Errorf("unsupported RDP client %q", options.Client)
	}

	address := net.JoinHostPort(
		connection.Endpoint.Host,
		strconv.FormatUint(uint64(connection.Endpoint.Port), 10),
	)
	args := []string{"/v:" + address}
	if connection.Endpoint.User != "" {
		args = append(args, "/u:"+connection.Endpoint.User)
	}
	if options.Fullscreen {
		args = append(args, "/f")
	}
	if options.IgnoreCertificate {
		args = append(args, "/cert:ignore")
	}

	return Invocation{Program: c.Name(), Args: args}, nil
}
