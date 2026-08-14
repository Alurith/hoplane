package config

import "github.com/Alurith/hoplane/internal/domain"

const CurrentVersion = 1

// File is the persisted hoplane configuration.
type File struct {
	Version     int     `yaml:"version"`
	Connections []Entry `yaml:"connections"`
}

// Entry is the user-facing, declarative representation of a connection.
type Entry struct {
	Name        string   `yaml:"name"`
	Protocol    string   `yaml:"protocol"`
	Host        string   `yaml:"host"`
	Port        *uint16  `yaml:"port,omitempty"`
	User        string   `yaml:"user,omitempty"`
	Description string   `yaml:"description,omitempty"`
	Tags        []string `yaml:"tags,omitempty"`
}

func NewFile() File {
	return File{Version: CurrentVersion, Connections: []Entry{}}
}

func (e Entry) Candidate(source domain.SourceRef) domain.Candidate {
	return domain.Candidate{
		Name:        e.Name,
		Protocol:    e.Protocol,
		Host:        e.Host,
		Port:        e.Port,
		User:        e.User,
		Description: e.Description,
		Tags:        e.Tags,
		Source:      source,
	}
}

func EntryFromConnection(connection domain.Connection) Entry {
	port := connection.Endpoint.Port
	return Entry{
		Name:        connection.Name,
		Protocol:    string(connection.Endpoint.Protocol),
		Host:        connection.Endpoint.Host,
		Port:        &port,
		User:        connection.Endpoint.User,
		Description: connection.Description,
		Tags:        connection.Tags,
	}
}
