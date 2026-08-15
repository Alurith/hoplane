package output

import (
	"encoding/json"
	"io"

	"github.com/Alurith/hoplane/internal/catalog"
	"github.com/Alurith/hoplane/internal/domain"
	"github.com/Alurith/hoplane/internal/rdpoptions"
	"github.com/Alurith/hoplane/internal/sshoptions"
)

const Version = 1

type Connection struct {
	Name        string              `json:"name"`
	Protocol    string              `json:"protocol"`
	Host        string              `json:"host"`
	Port        uint16              `json:"port"`
	User        string              `json:"user,omitempty"`
	Description string              `json:"description,omitempty"`
	Tags        []string            `json:"tags,omitempty"`
	Source      domain.SourceRef    `json:"source"`
	Options     domain.Options      `json:"options,omitempty"`
	SSHConfig   *SSHConfigReference `json:"ssh_config,omitempty"`
	Security    *SecurityPosture    `json:"security,omitempty"`
}

type SecurityPosture struct {
	CertificateValidation string `json:"certificate_validation"`
}

type SSHConfigReference struct {
	File  string `json:"file"`
	Alias string `json:"alias"`
}

type Warning struct {
	Source  string `json:"source"`
	Message string `json:"message"`
}

type ListResponse struct {
	Version     int          `json:"version"`
	Connections []Connection `json:"connections"`
	Warnings    []Warning    `json:"warnings"`
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
		SSHConfig:   publicSSHConfig(connection),
	}
	if connection.Endpoint.Protocol == domain.ProtocolRDP {
		validation := "client-default"
		options, err := rdpoptions.Decode(connection.Options)
		if err != nil || (options.Client != "" && options.Client != "xfreerdp") {
			validation = "invalid-config"
		} else if options.IgnoreCertificate {
			validation = "ignored"
		}
		result.Security = &SecurityPosture{CertificateValidation: validation}
	}
	return result
}

func publicOptions(connection domain.Connection) domain.Options {
	switch connection.Endpoint.Protocol {
	case domain.ProtocolSSH:
		options, err := sshoptions.Decode(connection.Options)
		if err != nil {
			return nil
		}
		values := make(map[string]string, 2)
		if options.IdentityFile != "" {
			values[sshoptions.IdentityFile] = options.IdentityFile
		}
		if options.ProxyJump != "" {
			values[sshoptions.ProxyJump] = options.ProxyJump
		}
		if len(values) == 0 {
			return nil
		}
		return domain.Options{sshoptions.Namespace: values}
	case domain.ProtocolRDP:
		options, err := rdpoptions.Decode(connection.Options)
		if err != nil || (options.Client != "" && options.Client != "xfreerdp") {
			return nil
		}
		return rdpoptions.Encode(options)
	default:
		return nil
	}
}

func publicSSHConfig(connection domain.Connection) *SSHConfigReference {
	if connection.Endpoint.Protocol != domain.ProtocolSSH {
		return nil
	}
	values := connection.Metadata[sshoptions.Namespace]
	if values == nil || values[sshoptions.ConfigFile] == "" || values[sshoptions.HostAlias] == "" {
		return nil
	}
	return &SSHConfigReference{
		File:  values[sshoptions.ConfigFile],
		Alias: values[sshoptions.HostAlias],
	}
}

func WriteList(w io.Writer, value catalog.Catalog) error {
	connections := make([]Connection, 0, len(value.Connections))
	for _, connection := range value.Connections {
		connections = append(connections, FromDomain(connection))
	}
	warnings := make([]Warning, 0, len(value.Warnings))
	for _, warning := range value.Warnings {
		warnings = append(warnings, Warning{Source: warning.Source, Message: warning.Message})
	}
	return encode(w, ListResponse{
		Version:     Version,
		Connections: connections,
		Warnings:    warnings,
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
