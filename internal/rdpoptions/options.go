// Package rdpoptions contains RDP-specific option names and decoding. It is
// deliberately outside domain so the common model does not know RDP fields.
package rdpoptions

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/Alurith/hoplane/internal/domain"
)

const Namespace = "rdp"

const (
	Client            = "client"
	Fullscreen        = "fullscreen"
	IgnoreCertificate = "ignore_certificate"
)

// Options is the RDP-specific interpretation of domain.Options.
type Options struct {
	Client            string
	Fullscreen        bool
	IgnoreCertificate bool
}

// Decode extracts and validates options in the RDP namespace. Other
// namespaces are intentionally ignored so protocol-specific options can
// coexist in the common option set.
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
		case Client:
			options.Client = value
		case Fullscreen:
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				return Options{}, fmt.Errorf("RDP option %q must be a boolean: %w", key, err)
			}
			options.Fullscreen = parsed
		case IgnoreCertificate:
			parsed, err := strconv.ParseBool(value)
			if err != nil {
				return Options{}, fmt.Errorf("RDP option %q must be a boolean: %w", key, err)
			}
			options.IgnoreCertificate = parsed
		default:
			return Options{}, fmt.Errorf("unsupported RDP option %q", key)
		}
	}
	return options, nil
}

// Encode converts RDP-specific options to the protocol-neutral namespaced
// representation. False boolean values are omitted because absence has the
// same meaning as false.
func Encode(options Options) domain.Options {
	values := make(map[string]string, 3)
	if options.Client != "" {
		values[Client] = options.Client
	}
	if options.Fullscreen {
		values[Fullscreen] = strconv.FormatBool(options.Fullscreen)
	}
	if options.IgnoreCertificate {
		values[IgnoreCertificate] = strconv.FormatBool(options.IgnoreCertificate)
	}
	if len(values) == 0 {
		return nil
	}
	return domain.Options{Namespace: values}
}

func validateValue(key, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("RDP option %q cannot be empty", key)
	}
	for _, r := range value {
		if r == 0 || unicode.IsControl(r) {
			return fmt.Errorf("RDP option %q contains a control character", key)
		}
	}
	return nil
}
