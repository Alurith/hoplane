// Package sshoptions contains SSH-specific option names and decoding. It is
// deliberately outside domain so the common model does not know SSH fields.
package sshoptions

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/Alurith/hoplane/internal/domain"
)

const Namespace = "ssh"

const (
	IdentityFile = "identity_file"
	ProxyJump    = "proxy_jump"

	// ConfigFile and HostAlias are internal references used by SSHConfigSource.
	ConfigFile = "config_file"
	HostAlias  = "host_alias"
)

// Options is the SSH-specific interpretation of domain.Options.
type Options struct {
	IdentityFile string
	ProxyJump    string
	ConfigFile   string
	HostAlias    string
}

// Decode extracts and validates options in the SSH namespace. Other
// namespaces are intentionally ignored so future connectors can coexist in
// the common option set.
func Decode(all domain.Options) (Options, error) {
	values := all[Namespace]
	var options Options
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := values[key]
		if err := validateValue(key, value); err != nil {
			return Options{}, err
		}
		switch key {
		case IdentityFile:
			options.IdentityFile = value
		case ProxyJump:
			options.ProxyJump = value
		case ConfigFile:
			options.ConfigFile = value
		case HostAlias:
			options.HostAlias = value
		default:
			return Options{}, fmt.Errorf("unsupported SSH option %q", key)
		}
	}
	if options.HostAlias != "" && options.ConfigFile == "" {
		return Options{}, fmt.Errorf("SSH option %q requires %q", HostAlias, ConfigFile)
	}
	if options.HostAlias != "" {
		if strings.HasPrefix(options.HostAlias, "-") || strings.ContainsAny(options.HostAlias, "@:/\\[]") {
			return Options{}, fmt.Errorf("SSH option %q is not a valid host alias", HostAlias)
		}
	}
	return options, nil
}

// ConfigReference creates the opaque references carried by candidates from
// an SSH config source. The actual SSH config remains owned by OpenSSH.
func ConfigReference(path, alias string) domain.Options {
	return domain.Options{
		Namespace: {
			ConfigFile: path,
			HostAlias:  alias,
		},
	}
}

func validateValue(key, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("SSH option %q cannot be empty", key)
	}
	for _, r := range value {
		if r == 0 || unicode.IsControl(r) {
			return fmt.Errorf("SSH option %q contains a control character", key)
		}
	}
	return nil
}
