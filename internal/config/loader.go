package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func DefaultPath() (string, error) {
	directory, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config directory: %w", err)
	}
	return filepath.Join(directory, "hoplane", "config.yaml"), nil
}

func Load(path string) (File, error) {
	contents, err := os.ReadFile(path)
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

	if file.Version != CurrentVersion {
		return File{}, fmt.Errorf("config %q has unsupported version %d, want %d", path, file.Version, CurrentVersion)
	}
	if file.Connections == nil {
		file.Connections = []Entry{}
	}
	return file, nil
}

func Save(path string, file File) error {
	if file.Version == 0 {
		file.Version = CurrentVersion
	}
	if file.Version != CurrentVersion {
		return fmt.Errorf("cannot save unsupported config version %d", file.Version)
	}
	if file.Connections == nil {
		file.Connections = []Entry{}
	}

	contents, err := yaml.Marshal(file)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}

	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
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

	if err := os.Rename(temporaryPath, path); err != nil {
		// Windows does not replace an existing file with Rename. The fallback
		// keeps the write usable on that platform, while Unix stays atomic.
		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return fmt.Errorf("replace config: %w", err)
		}
		if renameErr := os.Rename(temporaryPath, path); renameErr != nil {
			return fmt.Errorf("replace config: %w", renameErr)
		}
	}
	return nil
}
