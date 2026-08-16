//go:build linux

package connector

import (
	"net"
	"strconv"

	"github.com/Alurith/hoplane/internal/domain"
	"github.com/Alurith/hoplane/internal/rdpoptions"
)

const xfreerdp3ClientID = "xfreerdp3"

type xfreerdp3Client struct{}

func (xfreerdp3Client) ID() string {
	return xfreerdp3ClientID
}

func (xfreerdp3Client) Program() string {
	return "xfreerdp3"
}

func (xfreerdp3Client) Capabilities() rdpClientCapabilities {
	return rdpClientCapabilities{
		Fullscreen:        true,
		IgnoreCertificate: true,
	}
}

func (xfreerdp3Client) Plan(
	connection domain.Connection,
	options rdpoptions.Options,
) ([]string, error) {
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
	return args, nil
}
