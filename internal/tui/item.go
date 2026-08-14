package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/list"

	"github.com/Alurith/hoplane/internal/domain"
)

var _ list.DefaultItem = Item{}

type Item struct {
	connection domain.Connection
}

func NewItem(connection domain.Connection) Item {
	return Item{connection: connection}
}

func (i Item) FilterValue() string { return i.connection.Name }
func (i Item) Title() string       { return i.connection.Name }

func (i Item) Description() string {
	endpoint := i.connection.Endpoint
	address := endpoint.Address()
	if endpoint.User != "" {
		address = fmt.Sprintf("%s@%s", endpoint.User, address)
	}
	lines := []string{fmt.Sprintf("%s://%s", endpoint.Protocol, address)}
	if i.connection.Description != "" {
		lines = append(lines, i.connection.Description)
	}
	if len(i.connection.Tags) > 0 {
		lines = append(lines, "#"+strings.Join(i.connection.Tags, " #"))
	}
	return strings.Join(lines, "\n")
}

func (i Item) Connection() domain.Connection { return i.connection }
