package connector

import "github.com/Alurith/hoplane/internal/domain"

func rdpConnection() domain.Connection {
	return domain.Connection{
		Name: "office",
		Endpoint: domain.Endpoint{
			Protocol: domain.ProtocolRDP,
			Host:     "desktop.example.com",
			Port:     3389,
			User:     "alice",
		},
	}
}
