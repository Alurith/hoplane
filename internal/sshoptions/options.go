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

	// ConfigFile and HostAlias are source metadata used by SSHConfigSource.
	// They are deliberately not valid persisted options.
	ConfigFile = "config_file"
	HostAlias  = "host_alias"
)

// Options is the SSH-specific interpretation of domain.Options.
type Options struct {
	IdentityFile string
	ProxyJump    string
}

// ConfigReference identifies an alias owned by an OpenSSH configuration
// source. It is runtime metadata, not a user-editable option.
type ConfigReference struct {
	ConfigFile string
	HostAlias  string
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
		case ConfigFile, HostAlias:
			return Options{}, fmt.Errorf("SSH option %q is source metadata, not a persisted option", key)
		default:
			return Options{}, fmt.Errorf("unsupported SSH option %q", key)
		}
	}
	return options, nil
}

// NewConfigReference creates runtime metadata carried by candidates from an
// SSH config source. The actual SSH config remains owned by OpenSSH.
func NewConfigReference(path, alias string) domain.Metadata {
	return domain.Metadata{
		Namespace: {
			ConfigFile: path,
			HostAlias:  alias,
		},
	}
}

// DecodeMetadata validates the source-owned OpenSSH reference.
func DecodeMetadata(all domain.Metadata) (ConfigReference, error) {
	values := all[Namespace]
	path := values[ConfigFile]
	alias := values[HostAlias]
	if path == "" || alias == "" {
		return ConfigReference{}, fmt.Errorf("SSH metadata requires %q and %q", ConfigFile, HostAlias)
	}
	for key, value := range values {
		if key != ConfigFile && key != HostAlias {
			return ConfigReference{}, fmt.Errorf("unsupported SSH metadata %q", key)
		}
		if err := validateValue(key, value); err != nil {
			return ConfigReference{}, err
		}
	}
	if strings.HasPrefix(alias, "-") || strings.ContainsAny(alias, "@:/\\[]") {
		return ConfigReference{}, fmt.Errorf("SSH metadata %q is not a valid host alias", HostAlias)
	}
	return ConfigReference{ConfigFile: path, HostAlias: alias}, nil
}

func validateValue(key, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("SSH option %q cannot be empty", key)
	}
	for _, r := range value {
		if r == 0 || unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return fmt.Errorf("SSH option %q contains an unsafe control character", key)
		}
	}
	return nil
}
