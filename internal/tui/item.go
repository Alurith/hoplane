package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/list"

	"github.com/Alurith/hoplane/internal/domain"
	"github.com/Alurith/hoplane/internal/rdpoptions"
	"github.com/Alurith/hoplane/internal/terminal"
)

var _ list.DefaultItem = Item{}

type Item struct {
	connection domain.Connection
}

func NewItem(connection domain.Connection) Item {
	return Item{connection: connection}
}

func (i Item) FilterValue() string { return terminal.EscapeControls(i.connection.Name) }
func (i Item) Title() string       { return terminal.EscapeControls(i.connection.Name) }

func (i Item) Description() string {
	endpoint := i.connection.Endpoint
	address := endpoint.Address()
	if endpoint.User != "" {
		address = fmt.Sprintf("%s@%s", terminal.EscapeControls(endpoint.User), address)
	}
	lines := []string{fmt.Sprintf("%s://%s", endpoint.Protocol, address)}
	if endpoint.Protocol == domain.ProtocolRDP {
		if options, err := rdpoptions.Decode(i.connection.Options); err == nil && options.IgnoreCertificate {
			lines = append(lines, "WARNING: RDP certificate validation disabled")
		}
	}
	if i.connection.Description != "" {
		lines = append(lines, terminal.EscapeControls(i.connection.Description))
	}
	if len(i.connection.Tags) > 0 {
		tags := make([]string, 0, len(i.connection.Tags))
		for _, tag := range i.connection.Tags {
			tags = append(tags, terminal.EscapeControls(tag))
		}
		lines = append(lines, "#"+strings.Join(tags, " #"))
	}
	return strings.Join(lines, "\n")
}

func (i Item) Connection() domain.Connection { return i.connection }
