package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Alurith/hoplane/internal/domain"
	"github.com/Alurith/hoplane/internal/rdpoptions"
	"github.com/Alurith/hoplane/internal/safeio"
	"gopkg.in/yaml.v3"
)

const (
	MaxBytes       int64 = 1 << 20
	MaxConnections       = 4096
)

var configFilePolicy = safeio.Policy{
	MaxBytes:       MaxBytes,
	RequireOwner:   true,
	RequirePrivate: true,
}

func DefaultPath() (string, error) {
	directory, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(directory, "hoplane", "config.yaml"), nil
}

func Load(path string) (File, error) {
	if err := safeio.ValidateAncestors(path); err != nil {
		return File{}, fmt.Errorf("validate config path: %w", err)
	}
	if err := validateConfigDirectoryForLoad(path); err != nil {
		return File{}, err
	}
	contents, err := safeio.ReadFile(path, configFilePolicy)
	if errors.Is(err, os.ErrNotExist) {
		return NewFile(), nil
	}
	if err != nil {
		return File{}, fmt.Errorf("read config %q: %w", path, err)
	}

	var file File
	decoder := yaml.NewDecoder(bytes.NewReader(contents))
	decoder.KnownFields(true)
	if err := decoder.Decode(&file); err != nil {
		return File{}, fmt.Errorf("parse config %q: %w", path, err)
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return File{}, fmt.Errorf("config %q contains multiple YAML documents", path)
		}
		return File{}, fmt.Errorf("parse config %q: %w", path, err)
	}

	if len(file.Connections) > MaxConnections {
		return File{}, fmt.Errorf("config %q contains too many connections (max %d)", path, MaxConnections)
	}
	if file.Version != CurrentVersion {
		return File{}, fmt.Errorf("config %q has unsupported version %d, want %d", path, file.Version, CurrentVersion)
	}
	if err := validatePersistedFile(file); err != nil {
		return File{}, fmt.Errorf("validate config %q: %w", path, err)
	}
	if file.Connections == nil {
		file.Connections = []Entry{}
	}
	return file, nil
}

func validateConfigDirectoryForLoad(path string) error {
	directory := filepath.Dir(path)
	err := safeio.ValidateDirectory(directory, safeio.Policy{
		RequireOwner:          true,
		RejectGroupOtherWrite: true,
	})
	if err == nil {
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("validate config directory: %w", err)
	}
	if _, statErr := os.Lstat(path); errors.Is(statErr, os.ErrNotExist) {
		return nil
	}
	return fmt.Errorf("validate config directory: %w", err)
}

func Save(path string, file File) error {
	if file.Version != CurrentVersion {
		return fmt.Errorf("cannot save unsupported config version %d", file.Version)
	}
	if file.Connections == nil {
		file.Connections = []Entry{}
	}
	if len(file.Connections) > MaxConnections {
		return fmt.Errorf("config contains too many connections (max %d)", MaxConnections)
	}
	if err := validatePersistedFile(file); err != nil {
		return fmt.Errorf("validate config: %w", err)
	}

	contents, err := yaml.Marshal(file)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if int64(len(contents)) > MaxBytes {
		return fmt.Errorf("encode config: %w", safeio.ErrTooLarge)
	}

	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err := safeio.ValidateAncestors(path); err != nil {
		return fmt.Errorf("validate config path: %w", err)
	}
	if err := safeio.EnsurePrivateDirectory(directory); err != nil {
		return fmt.Errorf("validate config directory: %w", err)
	}
	if err := safeio.Validate(path, configFilePolicy); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("validate existing config: %w", err)
	}

	temporary, err := os.CreateTemp(directory, ".config-*.yaml")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath) //nolint:errcheck

	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set config permissions: %w", err)
	}
	if _, err := temporary.Write(contents); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write config: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync config: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary config: %w", err)
	}

	if err := replaceFile(temporaryPath, path); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}

func validatePersistedFile(file File) error {
	for index, entry := range file.Connections {
		protocol, err := domain.ParseProtocol(entry.Protocol)
		if err != nil {
			return fmt.Errorf("connection %d: %w", index, err)
		}

		switch protocol {
		case domain.ProtocolSSH:
			if len(entry.Options) > 0 {
				return fmt.Errorf("connection %d: SSH options are not supported", index)
			}
		case domain.ProtocolRDP:
			if _, err := rdpoptions.Decode(entry.Options); err != nil {
				return fmt.Errorf("connection %d: %w", index, err)
			}
		}
	}
	return nil
}
