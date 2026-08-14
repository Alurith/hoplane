package domain

import (
	"fmt"
	"strings"
	"unicode"
)

// Protocol identifies the protocol a connector must use for an endpoint.
type Protocol string

const (
	ProtocolSSH = Protocol("ssh")
	ProtocolRDP = Protocol("rdp")
	ProtocolVNC = Protocol("vnc")
)

// ParseProtocol normalizes a protocol name and validates its syntax. Unknown
// protocols are accepted so that the catalog remains protocol-neutral; a
// connector can report unsupported protocols when connection execution is
// implemented.
func ParseProtocol(value string) (Protocol, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "", fmt.Errorf("protocol cannot be empty")
	}

	for index, r := range value {
		valid := unicode.IsLetter(r) || (index > 0 && (unicode.IsDigit(r) || r == '+' || r == '-' || r == '.'))
		if !valid {
			return "", fmt.Errorf("invalid protocol %q", value)
		}
	}

	return Protocol(value), nil
}

// DefaultPort returns the conventional port for a known protocol.
func DefaultPort(protocol Protocol) (uint16, bool) {
	switch protocol {
	case ProtocolSSH:
		return 22, true
	case ProtocolRDP:
		return 3389, true
	case ProtocolVNC:
		return 5900, true
	default:
		return 0, false
	}
}
