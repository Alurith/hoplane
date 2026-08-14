package output

import (
	"encoding/json"
	"io"

	"github.com/Alurith/hoplane/internal/catalog"
	"github.com/Alurith/hoplane/internal/domain"
)

const Version = 1

type Connection struct {
	Name        string   `json:"name"`
	Protocol    string   `json:"protocol"`
	Host        string   `json:"host"`
	Port        uint16   `json:"port"`
	User        string   `json:"user,omitempty"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
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
	return Connection{
		Name:        connection.Name,
		Protocol:    string(connection.Endpoint.Protocol),
		Host:        connection.Endpoint.Host,
		Port:        connection.Endpoint.Port,
		User:        connection.Endpoint.User,
		Description: connection.Description,
		Tags:        connection.Tags,
	}
}

func WriteList(w io.Writer, value catalog.Catalog) error {
	connections := make([]Connection, 0, len(value.Connections))
	for _, connection := range value.Connections {
		connections = append(connections, FromDomain(connection))
	}
	return encode(w, ListResponse{
		Version:     Version,
		Connections: connections,
		Warnings:    []Warning{},
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
