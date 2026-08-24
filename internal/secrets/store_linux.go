//go:build linux

package secrets

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

const secretToolPath = "/usr/bin/secret-tool"

const (
	lookupTimeout = 10 * time.Second
	writeTimeout  = 20 * time.Second
)

type secretServiceStore struct{}

func NewDefaultStore() Store {
	return &secretServiceStore{}
}

func (s *secretServiceStore) Lookup(ctx context.Context, ref Ref) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := ref.validate(); err != nil {
		return nil, err
	}
	max, err := MaxSecretBytesForProtocol(ref.Protocol)
	if err != nil {
		return nil, err
	}
	if !secretToolReady() {
		return nil, ErrUnavailable
	}
	present, err := secretServiceItemPresent(ctx, ref)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, ErrNotFound
	}
	commandContext, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()
	command := exec.CommandContext(commandContext, secretToolPath, "lookup", "service", "hoplane",
		"connection_id", ref.ConnectionID, "protocol", ref.Protocol, "kind", ref.Kind)
	var output limitedWriter
	defer output.Clear()
	output.limit = max + 1
	command.Stdout = &output
	command.Stderr = scrubWriter{}
	err = command.Run()
	if mapped := commandContextError(ctx, commandContext); mapped != nil {
		return nil, mapped
	}
	if err != nil {
		// Presence was established by search. A failed lookup is therefore a
		// backend/unlock error, never an absent item.
		return nil, ErrUnavailable
	}
	if output.exceeded {
		return nil, ErrUnavailable
	}

	// Copy before scrubbing output. The returned value is owned by the caller;
	// raw is always temporary, including the line terminator from secret-tool.
	raw := append([]byte(nil), output.Bytes()...)
	valueLength := len(raw)
	if valueLength > 0 && raw[valueLength-1] == '\n' {
		valueLength--
	}
	value := append([]byte(nil), raw[:valueLength]...)
	Clear(raw)
	if err := ValidateSecretForProtocol(ref.Protocol, value); err != nil {
		Clear(value)
		return nil, fmt.Errorf("stored secret is invalid: %w", err)
	}
	return value, nil
}

func (s *secretServiceStore) Store(ctx context.Context, ref Ref, value []byte) error {
	if err := ref.validate(); err != nil {
		return err
	}
	if err := ValidateSecretForProtocol(ref.Protocol, value); err != nil {
		return err
	}
	if !secretToolReady() {
		return ErrUnavailable
	}
	if err := secretServiceAvailable(ctx); err != nil {
		return err
	}
	commandContext, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	command := exec.CommandContext(commandContext, secretToolPath, "store",
		"--label=Hoplane login password", "service", "hoplane",
		"connection_id", ref.ConnectionID, "protocol", ref.Protocol, "kind", ref.Kind)
	command.Stdin = bytes.NewReader(value)
	command.Stdout = io.Discard
	command.Stderr = scrubWriter{}
	err := command.Run()
	if mapped := commandContextError(ctx, commandContext); mapped != nil {
		return mapped
	}
	if err != nil {
		return ErrUnavailable
	}
	return nil
}

func (s *secretServiceStore) Delete(ctx context.Context, ref Ref) error {
	if err := ref.validate(); err != nil {
		return err
	}
	if !secretToolReady() {
		return ErrUnavailable
	}
	present, err := secretServiceItemPresent(ctx, ref)
	if err != nil {
		return err
	}
	if !present {
		return nil
	}

	commandContext, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()
	command := exec.CommandContext(commandContext, secretToolPath, "clear", "service", "hoplane",
		"connection_id", ref.ConnectionID, "protocol", ref.Protocol, "kind", ref.Kind)
	command.Stdout = io.Discard
	command.Stderr = scrubWriter{}
	err = command.Run()
	if err == nil {
		return nil
	}
	if mapped := commandContextError(ctx, commandContext); mapped != nil {
		return mapped
	}
	// A failed clear is not treated as absence: the lookup above established
	// that the item existed and the backend may have failed in the meantime.
	return ErrUnavailable
}

func (s *secretServiceStore) Has(ctx context.Context, ref Ref) (bool, error) {
	value, err := s.Lookup(ctx, ref)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	Clear(value)
	return true, nil
}

func secretServiceAvailable(ctx context.Context) error {
	_, err := secretServiceSearch(ctx, "search", "--all", "service", "hoplane")
	return err
}

func secretServiceItemPresent(ctx context.Context, ref Ref) (bool, error) {
	return secretServiceSearch(ctx, "search", "--all", "--unlock", "service", "hoplane",
		"connection_id", ref.ConnectionID, "protocol", ref.Protocol, "kind", ref.Kind)
}

func secretServiceSearch(ctx context.Context, args ...string) (bool, error) {
	commandContext, cancel := context.WithTimeout(ctx, lookupTimeout)
	defer cancel()
	command := exec.CommandContext(commandContext, secretToolPath, args...)
	var output presenceWriter
	command.Stdout = &output
	command.Stderr = scrubWriter{}
	if err := command.Run(); err != nil {
		if mapped := commandContextError(ctx, commandContext); mapped != nil {
			return false, mapped
		}
		return false, ErrUnavailable
	}
	return output.present, nil
}

type scrubWriter struct{}

func (scrubWriter) Write(value []byte) (int, error) {
	Clear(value)
	return len(value), nil
}

type presenceWriter struct {
	present bool
}

func (w *presenceWriter) Write(value []byte) (int, error) {
	if len(value) > 0 {
		w.present = true
	}
	Clear(value)
	return len(value), nil
}

func commandContextError(parent, commandContext context.Context) error {
	if parentErr := parent.Err(); parentErr != nil {
		return parentErr
	}
	if errors.Is(commandContext.Err(), context.DeadlineExceeded) {
		return ErrTimeout
	}
	if errors.Is(commandContext.Err(), context.Canceled) {
		return context.Canceled
	}
	return nil
}

type limitedWriter struct {
	bytes.Buffer
	limit    int
	exceeded bool
}

func (w *limitedWriter) Write(value []byte) (int, error) {
	if len(w.Bytes()) >= w.limit {
		w.exceeded = true
		return len(value), nil
	}
	remaining := w.limit - len(w.Bytes())
	if len(value) > remaining {
		_, _ = w.Buffer.Write(value[:remaining])
		w.exceeded = true
		return len(value), nil
	}
	return w.Buffer.Write(value)
}

func (w *limitedWriter) Clear() {
	Clear(w.Buffer.Bytes())
	w.Buffer.Reset()
	w.exceeded = false
}

func secretToolReady() bool {
	info, err := os.Lstat(secretToolPath)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Mode().Perm()&0o111 == 0 || info.Mode().Perm()&0o022 != 0 {
		return false
	}
	if !trustedOwner(info) {
		return false
	}
	for directory := filepath.Dir(secretToolPath); ; directory = filepath.Dir(directory) {
		directoryInfo, err := os.Lstat(directory)
		if err != nil || !directoryInfo.IsDir() || directoryInfo.Mode().Perm()&0o022 != 0 || !trustedOwner(directoryInfo) {
			return false
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			break
		}
	}
	return true
}

func trustedOwner(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}
	uid := uint64(stat.Uid)
	return uid == 0 || uid == uint64(os.Geteuid())
}
