package discovery

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/Alurith/hoplane/internal/domain"
	"github.com/Alurith/hoplane/internal/safeio"
	"github.com/Alurith/hoplane/internal/sshoptions"
)

const (
	sshDiscoveryTimeout = 3 * time.Second
	maxSSHFileBytes     = 1 << 20
	maxSSHTotalBytes    = 8 << 20
	maxSSHLineBytes     = 64 << 10
	maxSSHFiles         = 128
	maxSSHIncludeDepth  = 16
	maxSSHPatterns      = 256
	maxSSHMatches       = 512
	maxSSHAliases       = 4096
)

var sshFilePolicy = safeio.SSHConfigPolicy(maxSSHFileBytes)

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
	if ctx == nil {
		ctx = context.Background()
	}
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
	discoveryContext, cancel := context.WithTimeout(ctx, sshDiscoveryTimeout)
	defer cancel()
	aliases, err := discoverAliases(discoveryContext, path)
	if errors.Is(err, os.ErrNotExist) {
		return []domain.Candidate{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read SSH config %q: %w", path, err)
	}

	source := domain.SourceRef{Name: s.Name(), ID: path}
	candidates := make([]domain.Candidate, 0, len(aliases))
	for _, alias := range aliases {
		if err := discoveryContext.Err(); err != nil {
			return nil, err
		}
		candidates = append(candidates, domain.Candidate{
			Name:     alias,
			Protocol: string(domain.ProtocolSSH),
			Host:     alias,
			Source:   source,
			Metadata: sshoptions.NewConfigReference(path, alias),
		})
	}
	return candidates, nil
}

type sshParseState struct {
	ctx            context.Context
	includeBaseDir string
	visitedFiles   map[string]struct{}
	seenAliases    map[string]struct{}
	aliases        []string
	totalBytes     int64
	fileCount      int
	patternCount   int
	matchCount     int
}

func discoverAliases(ctx context.Context, path string) ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home directory for SSH Include: %w", err)
	}
	state := &sshParseState{
		ctx:            ctx,
		includeBaseDir: filepath.Join(home, ".ssh"),
		visitedFiles:   make(map[string]struct{}),
		seenAliases:    make(map[string]struct{}),
		aliases:        make([]string, 0),
	}
	if err := state.parseFile(path, 0); err != nil {
		return nil, err
	}
	return state.aliases, nil
}

func (s *sshParseState) parseFile(path string, depth int) error {
	if err := s.ctx.Err(); err != nil {
		return err
	}
	if depth > maxSSHIncludeDepth {
		return fmt.Errorf("SSH Include nesting exceeds %d levels", maxSSHIncludeDepth)
	}

	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve included SSH config %q: %w", path, err)
	}
	absolutePath = filepath.Clean(absolutePath)
	if err := safeio.Validate(absolutePath, sshFilePolicy); err != nil {
		return err
	}
	canonicalPath, err := filepath.EvalSymlinks(absolutePath)
	if err != nil {
		return fmt.Errorf("resolve SSH config %q: %w", absolutePath, err)
	}
	canonicalPath = filepath.Clean(canonicalPath)
	if _, seen := s.visitedFiles[canonicalPath]; seen {
		return nil
	}
	if s.fileCount >= maxSSHFiles {
		return fmt.Errorf("SSH config includes exceed %d files", maxSSHFiles)
	}
	s.fileCount++
	s.visitedFiles[canonicalPath] = struct{}{}

	remaining := maxSSHTotalBytes - s.totalBytes
	if remaining <= 0 {
		return fmt.Errorf("SSH config data exceeds %d bytes", maxSSHTotalBytes)
	}
	policy := sshFilePolicy
	if remaining < policy.MaxBytes {
		policy.MaxBytes = remaining
	}
	contents, err := safeio.ReadFile(canonicalPath, policy)
	if err != nil {
		return err
	}
	s.totalBytes += int64(len(contents))

	scanner := bufio.NewScanner(bytes.NewReader(contents))
	scanner.Buffer(make([]byte, 1024), maxSSHLineBytes)
	lineNumber := 0
	for scanner.Scan() {
		if err := s.ctx.Err(); err != nil {
			return err
		}
		lineNumber++
		fields, err := splitSSHFields(scanner.Text())
		if err != nil {
			return fmt.Errorf("%s:%d: %w", canonicalPath, lineNumber, err)
		}
		if len(fields) == 0 {
			continue
		}

		switch strings.ToLower(fields[0]) {
		case "host":
			if len(fields) == 1 {
				return fmt.Errorf("%s:%d: Host requires at least one pattern", canonicalPath, lineNumber)
			}
			for _, pattern := range fields[1:] {
				if !isConcreteAlias(pattern) {
					continue
				}
				key := strings.ToLower(pattern)
				if _, seen := s.seenAliases[key]; seen {
					continue
				}
				if len(s.aliases) >= maxSSHAliases {
					return fmt.Errorf("SSH config contains more than %d aliases", maxSSHAliases)
				}
				s.seenAliases[key] = struct{}{}
				s.aliases = append(s.aliases, pattern)
			}
		case "include":
			if err := s.parseIncludes(s.includeBaseDir, fields[1:], depth+1); err != nil {
				return fmt.Errorf("%s:%d: %w", canonicalPath, lineNumber, err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan %q: %w", canonicalPath, err)
	}
	return nil
}

func (s *sshParseState) parseIncludes(baseDirectory string, patterns []string, depth int) error {
	for _, pattern := range patterns {
		if err := s.ctx.Err(); err != nil {
			return err
		}
		if s.patternCount >= maxSSHPatterns {
			return fmt.Errorf("SSH config contains more than %d Include patterns", maxSSHPatterns)
		}
		s.patternCount++
		path, err := expandConfigPath(pattern, baseDirectory)
		if err != nil {
			return err
		}
		matches, err := limitedGlob(s.ctx, path, maxSSHMatches-s.matchCount)
		if err != nil {
			return fmt.Errorf("invalid Include pattern %q: %w", pattern, err)
		}
		sort.Strings(matches)
		s.matchCount += len(matches)
		for _, match := range matches {
			if err := s.parseFile(match, depth); err != nil {
				return err
			}
		}
	}
	return nil
}

func limitedGlob(ctx context.Context, pattern string, limit int) ([]string, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("glob match limit exceeded")
	}
	volume := filepath.VolumeName(pattern)
	rest := strings.TrimPrefix(pattern, volume)
	current := volume
	if filepath.IsAbs(pattern) {
		current += string(filepath.Separator)
		rest = strings.TrimLeft(rest, string(filepath.Separator))
	} else {
		current = "."
	}
	components := make([]string, 0)
	for _, component := range strings.Split(rest, string(filepath.Separator)) {
		if component != "" {
			components = append(components, component)
		}
	}

	matches := make([]string, 0)
	var walk func(string, int) error
	walk = func(path string, index int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if index == len(components) {
			if err := safeio.Validate(path, sshFilePolicy); errors.Is(err, os.ErrNotExist) {
				return nil
			} else if err != nil {
				return err
			}
			if len(matches) >= limit {
				return fmt.Errorf("glob match limit exceeded")
			}
			matches = append(matches, filepath.Clean(path))
			return nil
		}

		component := components[index]
		if component == "." {
			return walk(path, index+1)
		}
		if component == ".." {
			return walk(filepath.Join(path, component), index+1)
		}
		if !strings.ContainsAny(component, "*?[") {
			next := filepath.Join(path, component)
			if _, err := os.Lstat(next); errors.Is(err, os.ErrNotExist) {
				return nil
			} else if err != nil {
				return err
			}
			return walk(next, index+1)
		}

		directory, err := safeio.OpenDirectory(path, sshFilePolicy)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			return err
		}
		defer directory.Close() //nolint:errcheck
		for {
			if err := ctx.Err(); err != nil {
				return err
			}
			entries, err := directory.ReadDir(64)
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return err
			}
			for _, entry := range entries {
				matched, err := filepath.Match(component, entry.Name())
				if err != nil {
					return err
				}
				if matched {
					if err := walk(filepath.Join(path, entry.Name()), index+1); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
	if err := walk(current, 0); err != nil {
		return nil, err
	}
	return matches, nil
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
		if unicode.IsSpace(r) || unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
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
