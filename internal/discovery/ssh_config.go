package discovery

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/Alurith/hoplane/internal/domain"
	"github.com/Alurith/hoplane/internal/sshoptions"
)

// SSHConfigSource exposes concrete aliases from an OpenSSH config. It only
// enumerates aliases; the ssh client remains authoritative for config
// semantics such as HostName, User, Port, Include and ProxyJump.
type SSHConfigSource struct {
	path string
}

func NewSSHConfigSource(path string) SSHConfigSource {
	return SSHConfigSource{path: filepath.Clean(path)}
}

func DefaultSSHConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".ssh", "config"), nil
}

func (SSHConfigSource) Name() string { return "ssh-config" }

func (s SSHConfigSource) Discover(ctx context.Context) ([]domain.Candidate, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.path == "" || s.path == "." {
		return nil, fmt.Errorf("SSH config path cannot be empty")
	}

	path, err := filepath.Abs(s.path)
	if err != nil {
		return nil, fmt.Errorf("resolve SSH config path %q: %w", s.path, err)
	}
	aliases, err := discoverAliases(ctx, path)
	if errors.Is(err, os.ErrNotExist) {
		return []domain.Candidate{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read SSH config %q: %w", path, err)
	}

	source := domain.SourceRef{Name: s.Name(), ID: path}
	candidates := make([]domain.Candidate, 0, len(aliases))
	for _, alias := range aliases {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		candidates = append(candidates, domain.Candidate{
			Name:     alias,
			Protocol: string(domain.ProtocolSSH),
			Host:     alias,
			Source:   source,
			Options:  sshoptions.ConfigReference(path, alias),
		})
	}
	return candidates, nil
}

func discoverAliases(ctx context.Context, path string) ([]string, error) {
	aliases := make([]string, 0)
	seenAliases := make(map[string]struct{})
	visitedFiles := make(map[string]struct{})
	if err := parseSSHConfigFile(ctx, path, visitedFiles, seenAliases, &aliases); err != nil {
		return nil, err
	}
	return aliases, nil
}

func parseSSHConfigFile(
	ctx context.Context,
	path string,
	visitedFiles map[string]struct{},
	seenAliases map[string]struct{},
	aliases *[]string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve included SSH config %q: %w", path, err)
	}
	absolutePath = filepath.Clean(absolutePath)
	if _, seen := visitedFiles[absolutePath]; seen {
		return nil
	}
	visitedFiles[absolutePath] = struct{}{}

	file, err := os.Open(absolutePath)
	if err != nil {
		return err
	}
	defer file.Close() //nolint:errcheck

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		if err := ctx.Err(); err != nil {
			return err
		}
		fields, err := splitSSHFields(scanner.Text())
		if err != nil {
			return fmt.Errorf("%s:%d: %w", absolutePath, lineNumber, err)
		}
		if len(fields) == 0 {
			continue
		}

		switch strings.ToLower(fields[0]) {
		case "host":
			if len(fields) == 1 {
				return fmt.Errorf("%s:%d: Host requires at least one pattern", absolutePath, lineNumber)
			}
			for _, pattern := range fields[1:] {
				if !isConcreteAlias(pattern) {
					continue
				}
				key := strings.ToLower(pattern)
				if _, seen := seenAliases[key]; seen {
					continue
				}
				seenAliases[key] = struct{}{}
				*aliases = append(*aliases, pattern)
			}
		case "include":
			if err := parseIncludes(ctx, filepath.Dir(absolutePath), fields[1:], visitedFiles, seenAliases, aliases); err != nil {
				return fmt.Errorf("%s:%d: %w", absolutePath, lineNumber, err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan %q: %w", absolutePath, err)
	}
	return nil
}

func parseIncludes(
	ctx context.Context,
	baseDirectory string,
	patterns []string,
	visitedFiles map[string]struct{},
	seenAliases map[string]struct{},
	aliases *[]string,
) error {
	for _, pattern := range patterns {
		if err := ctx.Err(); err != nil {
			return err
		}
		path, err := expandConfigPath(pattern, baseDirectory)
		if err != nil {
			return err
		}
		matches, err := filepath.Glob(path)
		if err != nil {
			return fmt.Errorf("invalid Include pattern %q: %w", pattern, err)
		}
		sort.Strings(matches)
		for _, match := range matches {
			if err := parseSSHConfigFile(ctx, match, visitedFiles, seenAliases, aliases); err != nil {
				return err
			}
		}
	}
	return nil
}

func expandConfigPath(value, baseDirectory string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("Include path cannot be empty")
	}
	if value == "~" || strings.HasPrefix(value, "~/") || strings.HasPrefix(value, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory for Include: %w", err)
		}
		value = filepath.Join(home, strings.TrimLeft(value[1:], `/\`))
	} else {
		value = filepath.FromSlash(value)
	}
	if !filepath.IsAbs(value) {
		value = filepath.Join(baseDirectory, value)
	}
	return filepath.Clean(value), nil
}

func isConcreteAlias(value string) bool {
	if value == "" {
		return false
	}
	if strings.ContainsAny(value, "*?!@:/\\[]") || strings.HasPrefix(value, "-") {
		return false
	}
	for _, r := range value {
		if unicode.IsSpace(r) || unicode.IsControl(r) {
			return false
		}
	}
	return true
}

// splitSSHFields handles the quoting needed for Host and Include lines. It
// intentionally does not attempt to implement all OpenSSH configuration
// semantics; those belong to the ssh client.
func splitSSHFields(line string) ([]string, error) {
	fields := make([]string, 0)
	var field strings.Builder
	var quote rune
	escaped := false
	flush := func() {
		if field.Len() > 0 {
			fields = append(fields, field.String())
			field.Reset()
		}
	}

	for _, r := range line {
		if escaped {
			field.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' && quote != '\'' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				field.WriteRune(r)
			}
			continue
		}
		switch r {
		case '\'', '"':
			quote = r
		case '#':
			flush()
			return fields, nil
		case ' ', '\t', '\r':
			flush()
		default:
			field.WriteRune(r)
		}
	}
	if escaped {
		field.WriteRune('\\')
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quoted value")
	}
	flush()
	return fields, nil
}

var _ Source = SSHConfigSource{}
