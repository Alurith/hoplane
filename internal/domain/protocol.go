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
)

// ParseProtocol normalizes a protocol name and validates that it is
// implemented by the application.
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

	protocol := Protocol(value)
	switch protocol {
	case ProtocolSSH, ProtocolRDP:
		return protocol, nil
	default:
		return "", fmt.Errorf("unsupported protocol %q", protocol)
	}
}

// DefaultPort returns the conventional port for a known protocol.
func DefaultPort(protocol Protocol) (uint16, bool) {
	switch protocol {
	case ProtocolSSH:
		return 22, true
	case ProtocolRDP:
		return 3389, true
	default:
		return 0, false
	}
}
