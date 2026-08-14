package domain

import (
	"fmt"
	"net/netip"
	"sort"
	"strings"
	"unicode"
)

// SourceRef identifies the source that produced an endpoint.
type SourceRef struct {
	Name string `json:"name"`
	ID   string `json:"id,omitempty"`
}

// Options is a protocol-neutral, namespaced option set. The domain preserves
// it without interpreting the values; protocol adapters own their semantics.
type Options map[string]map[string]string

// CloneOptions returns an independent copy of an option set.
func CloneOptions(options Options) Options {
	if options == nil {
		return nil
	}

	clone := make(Options, len(options))
	for namespace, values := range options {
		if values == nil {
			clone[namespace] = nil
			continue
		}
		clone[namespace] = make(map[string]string, len(values))
		for key, value := range values {
			clone[namespace][key] = value
		}
	}
	return clone
}

// Candidate is a source-specific endpoint before normalization.
type Candidate struct {
	Name        string
	Protocol    string
	Host        string
	Port        *uint16
	User        string
	Description string
	Tags        []string
	Source      SourceRef
	Options     Options
}

// Endpoint is the normalized, protocol-neutral target used by the catalog and
// picker.
type Endpoint struct {
	Protocol Protocol
	Host     string
	Port     uint16
	User     string
	Source   SourceRef
}

// Connection is the user-facing catalog entry. It gives a named endpoint
// metadata that is useful to both the CLI and the picker.
type Connection struct {
	Name        string
	Endpoint    Endpoint
	Description string
	Tags        []string
	Sources     []SourceRef
	Options     Options
}

// NormalizeCandidate validates and normalizes a source candidate.
func NormalizeCandidate(candidate Candidate) (Connection, error) {
	name := strings.TrimSpace(candidate.Name)
	if name == "" {
		return Connection{}, fmt.Errorf("name cannot be empty")
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return Connection{}, fmt.Errorf("name %q contains a control character", name)
		}
	}

	protocol, err := ParseProtocol(candidate.Protocol)
	if err != nil {
		return Connection{}, fmt.Errorf("connection %q: %w", name, err)
	}

	host, err := normalizeHost(candidate.Host)
	if err != nil {
		return Connection{}, fmt.Errorf("connection %q: %w", name, err)
	}

	port, err := normalizePort(protocol, candidate.Port)
	if err != nil {
		return Connection{}, fmt.Errorf("connection %q: %w", name, err)
	}

	user := strings.TrimSpace(candidate.User)
	if strings.ContainsAny(user, "\r\n") {
		return Connection{}, fmt.Errorf("connection %q: user cannot contain a newline", name)
	}

	source := candidate.Source
	if source.Name == "" {
		source = SourceRef{Name: "unknown"}
	}

	return Connection{
		Name: name,
		Endpoint: Endpoint{
			Protocol: protocol,
			Host:     host,
			Port:     port,
			User:     user,
			Source:   source,
		},
		Description: strings.TrimSpace(candidate.Description),
		Tags:        normalizeTags(candidate.Tags),
		Sources:     []SourceRef{source},
		Options:     CloneOptions(candidate.Options),
	}, nil
}

func normalizeHost(value string) (string, error) {
	host := strings.TrimSpace(value)
	if host == "" {
		return "", fmt.Errorf("host cannot be empty")
	}
	if strings.ContainsAny(host, "/\\") {
		return "", fmt.Errorf("host %q contains invalid characters", host)
	}
	for _, r := range host {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return "", fmt.Errorf("host %q contains invalid characters", host)
		}
	}

	if strings.HasPrefix(host, "[") && strings.HasSuffix(host, "]") {
		host = strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
	}
	if address, err := netip.ParseAddr(host); err == nil {
		return address.String(), nil
	}
	if strings.Contains(host, ":") {
		return "", fmt.Errorf("invalid host %q", host)
	}

	return strings.ToLower(host), nil
}

func normalizePort(protocol Protocol, port *uint16) (uint16, error) {
	if port != nil {
		if *port == 0 {
			return 0, fmt.Errorf("port must be between 1 and 65535")
		}
		return *port, nil
	}
	if defaultPort, ok := DefaultPort(protocol); ok {
		return defaultPort, nil
	}
	return 0, fmt.Errorf("port is required for protocol %q", protocol)
}

func normalizeTags(tags []string) []string {
	seen := make(map[string]struct{}, len(tags))
	result := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, ok := seen[tag]; ok {
			continue
		}
		seen[tag] = struct{}{}
		result = append(result, tag)
	}
	sort.Strings(result)
	return result
}

// Address returns a display-friendly host:port value.
func (e Endpoint) Address() string {
	if strings.Contains(e.Host, ":") {
		return fmt.Sprintf("[%s]:%d", e.Host, e.Port)
	}
	return fmt.Sprintf("%s:%d", e.Host, e.Port)
}
