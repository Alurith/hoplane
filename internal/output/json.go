package output

import (
	"encoding/json"
	"io"

	"github.com/Alurith/hoplane/internal/domain"
	"github.com/Alurith/hoplane/internal/rdpoptions"
)

const Version = 2

type Connection struct {
	Name        string           `json:"name"`
	Protocol    string           `json:"protocol"`
	Host        string           `json:"host"`
	Port        uint16           `json:"port"`
	User        string           `json:"user,omitempty"`
	Description string           `json:"description,omitempty"`
	Tags        []string         `json:"tags,omitempty"`
	Source      domain.SourceRef `json:"source"`
	Options     domain.Options   `json:"options,omitempty"`
	Security    *SecurityPosture `json:"security,omitempty"`
}

type SecurityPosture struct {
	CertificateValidation string `json:"certificate_validation"`
}

type ListResponse struct {
	Version     int          `json:"version"`
	Connections []Connection `json:"connections"`
}

type ConnectionResponse struct {
	Version    int        `json:"version"`
	Connection Connection `json:"connection"`
}

func FromDomain(connection domain.Connection) Connection {
	result := Connection{
		Name:        connection.Name,
		Protocol:    string(connection.Endpoint.Protocol),
		Host:        connection.Endpoint.Host,
		Port:        connection.Endpoint.Port,
		User:        connection.Endpoint.User,
		Description: connection.Description,
		Tags:        connection.Tags,
		Source:      connection.Endpoint.Source,
		Options:     publicOptions(connection),
	}
	if connection.Endpoint.Protocol == domain.ProtocolRDP {
		validation := "client-default"
		options, err := rdpoptions.Decode(connection.Options)
		if err != nil {
			validation = "invalid-config"
		} else if options.IgnoreCertificate {
			validation = "ignored"
		}
		result.Security = &SecurityPosture{CertificateValidation: validation}
	}
	return result
}

func publicOptions(connection domain.Connection) domain.Options {
	if connection.Endpoint.Protocol != domain.ProtocolRDP {
		return nil
	}
	options, err := rdpoptions.Decode(connection.Options)
	if err != nil {
		return nil
	}
	return rdpoptions.Encode(options)
}

func WriteList(w io.Writer, connections []domain.Connection) error {
	result := make([]Connection, 0, len(connections))
	for _, connection := range connections {
		result = append(result, FromDomain(connection))
	}
	return encode(w, ListResponse{
		Version:     Version,
		Connections: result,
	})
}

func WriteConnection(w io.Writer, value domain.Connection) error {
	return encode(w, ConnectionResponse{
		Version:    Version,
		Connection: FromDomain(value),
	})
}

func encode(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
