//go:build linux

package connector

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

const (
	processHelperModeEnv = "HOPLANE_PROCESS_HELPER_MODE"
	processHelperPIDEnv  = "HOPLANE_PROCESS_HELPER_PID_FILE"
	processHelperWaitEnv = "HOPLANE_PROCESS_HELPER_WAIT"
)

func init() {
	switch os.Getenv(processHelperModeEnv) {
	case "parent":
		processHelperParent()
	case "descendant":
		if os.Getenv(processHelperWaitEnv) == "ignore-term" {
			signalIgnoreTERM()
		}
		for {
			time.Sleep(time.Second)
		}
	}
}

func signalIgnoreTERM() {
	// This is deliberately used only by the child helper so TERM cannot
	// substitute for the runner's SIGKILL escalation.
	signal.Ignore(syscall.SIGTERM)
}

func processHelperParent() {
	fd3 := os.NewFile(3, "helper-fd3")
	if fd3 == nil {
		os.Exit(2)
	}
	defer fd3.Close()
	child := exec.Command(os.Args[0], "-test.run=^TestProcessFD3Helper$")
	child.Env = append(os.Environ(), processHelperModeEnv+"=descendant")
	if waitMode := os.Getenv(processHelperWaitEnv); waitMode != "" {
		child.Env = append(child.Env, processHelperWaitEnv+"="+waitMode)
	}
	child.ExtraFiles = []*os.File{fd3}
	child.Stdout = nil
	child.Stderr = nil
	if err := child.Start(); err != nil {
		os.Exit(3)
	}
	if err := os.WriteFile(os.Getenv(processHelperPIDEnv), []byte(strconv.Itoa(child.Process.Pid)), 0o600); err != nil {
		_ = child.Process.Kill()
		os.Exit(4)
	}
	if os.Getenv(processHelperWaitEnv) != "" {
		for {
			time.Sleep(time.Second)
		}
	}
	os.Exit(0)
}

func TestExecRunnerRunWithSecretUsesDescriptorNotInvocation(t *testing.T) {
	var output bytes.Buffer
	payload := []byte("/v:host\n/p:private-value\n")
	wantPayload := string(payload)
	invocation := Invocation{Program: "/usr/bin/cat", Args: []string{"/dev/fd/3"}}
	request := SecretProcessRequest{Transport: SecretTransportRDPArgsFD, Payload: payload}
	if err := (ExecRunner{}).RunWithSecret(context.Background(), invocation, IO{Output: &output}, request); err != nil {
		t.Fatalf("RunWithSecret() error = %v", err)
	}
	if output.String() != wantPayload {
		t.Fatalf("output = %q, want %q", output.String(), wantPayload)
	}
	if strings.Contains(invocation.String(), wantPayload) {
		t.Fatal("payload appeared in invocation")
	}
	if !bytes.Equal(payload, make([]byte, len(payload))) {
		t.Fatal("payload was not cleared after the writer completed")
	}
}

func TestExecRunnerRunWithSecretSSHUsesPrivateAskpassFile(t *testing.T) {
	directory := t.TempDir()
	program := filepath.Join(directory, "ssh-helper")
	if err := os.WriteFile(program, []byte("#!/bin/sh\ncat \"$HOPLANE_ASKPASS_FILE\"\n"), 0o700); err != nil {
		t.Fatalf("write helper: %v", err)
	}
	payload := []byte("private-password")
	wantPayload := string(payload)
	var output bytes.Buffer
	if err := (ExecRunner{}).RunWithSecret(context.Background(), Invocation{Program: program}, IO{Output: &output}, SecretProcessRequest{
		Transport: SecretTransportSSHAskpass,
		Payload:   payload,
	}); err != nil {
		t.Fatalf("RunWithSecret() error = %v", err)
	}
	if output.String() != wantPayload {
		t.Fatalf("output = %q, want %q", output.String(), wantPayload)
	}
	if !bytes.Equal(payload, make([]byte, len(payload))) {
		t.Fatal("payload was not cleared after the process completed")
	}
}

func TestPrepareAskpassSecretUsesPrivateFileAndCleansUp(t *testing.T) {
	path, cleanup, err := prepareAskpassSecret([]byte("test-secret"))
	if err != nil {
		t.Fatalf("prepareAskpassSecret() error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat askpass secret: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("askpass secret permissions = %o, want 600", got)
	}
	if err := cleanup(); err != nil {
		t.Fatalf("cleanup askpass secret: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("askpass secret still exists, stat error = %v", err)
	}
}

func TestPrepareAskpassSecretRejectsUnsafeTempDirectory(t *testing.T) {
	root := t.TempDir()
	unsafe := filepath.Join(root, "unsafe")
	if err := os.Mkdir(unsafe, 0o777); err != nil {
		t.Fatalf("mkdir unsafe temp directory: %v", err)
	}
	if err := os.Chmod(unsafe, 0o777); err != nil {
		t.Fatalf("chmod unsafe temp directory: %v", err)
	}
	t.Setenv("TMPDIR", unsafe)
	if _, cleanup, err := prepareAskpassSecret([]byte("test-secret")); err == nil {
		t.Fatal("prepareAskpassSecret() accepted an unsafe temp directory")
	} else if cleanup != nil {
		t.Fatal("prepareAskpassSecret() returned cleanup after rejecting the directory")
	}
}

func TestRunWithSecretRejectsInvalidPayloadBeforeStart(t *testing.T) {
	payload := []byte("bad\x00payload")
	request := SecretProcessRequest{Transport: SecretTransportSSHAskpass, Payload: payload}
	err := (ExecRunner{}).RunWithSecret(context.Background(), Invocation{Program: "/usr/bin/true"}, IO{}, request)
	if err == nil || !strings.Contains(err.Error(), "NUL") {
		t.Fatalf("RunWithSecret() error = %v, want NUL rejection", err)
	}
	if !bytes.Equal(payload, make([]byte, len(payload))) {
		t.Fatal("invalid payload was not cleared")
	}
}

func TestRunWithSecretDescendantHoldingFD3DoesNotDeadlock(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "descendant.pid")
	t.Setenv(processHelperModeEnv, "parent")
	t.Setenv(processHelperPIDEnv, pidFile)
	t.Setenv(processHelperWaitEnv, "")
	program := safeTestProgram(t)
	payload := rdpTestPayload(1 << 20)
	result := make(chan error, 1)
	go func() {
		result <- (ExecRunner{}).RunWithSecret(context.Background(), Invocation{Program: program}, IO{}, SecretProcessRequest{
			Transport: SecretTransportRDPArgsFD,
			Payload:   payload,
		})
	}()
	select {
	case err := <-result:
		if err != nil && !errors.Is(err, os.ErrProcessDone) {
			t.Fatalf("RunWithSecret() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RunWithSecret() deadlocked with descendant holding FD3")
	}
	pid := readHelperPID(t, pidFile)
	waitForPIDExit(t, pid)
}

func TestRunWithSecretCancellationKillsProcessGroup(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "descendant.pid")
	t.Setenv(processHelperModeEnv, "parent")
	t.Setenv(processHelperPIDEnv, pidFile)
	t.Setenv(processHelperWaitEnv, "ignore-term")
	program := safeTestProgram(t)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	payload := rdpTestPayload(1 << 20)
	result := make(chan error, 1)
	go func() {
		result <- (ExecRunner{}).RunWithSecret(ctx, Invocation{Program: program}, IO{}, SecretProcessRequest{
			Transport: SecretTransportRDPArgsFD,
			Payload:   payload,
		})
	}()
	select {
	case err := <-result:
		if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
			t.Fatalf("RunWithSecret() error = %v, want context cancellation", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RunWithSecret() timed out during process-group cancellation")
	}
	pid := readHelperPID(t, pidFile)
	waitForPIDExit(t, pid)
}

func safeTestProgram(t *testing.T) string {
	t.Helper()
	directory, err := os.MkdirTemp(".", ".process-helper-")
	if err != nil {
		t.Fatalf("create helper directory: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(directory) })
	program, err := filepath.Abs(filepath.Join(directory, "process-helper"))
	if err != nil {
		t.Fatalf("resolve helper path: %v", err)
	}
	data, err := os.ReadFile(os.Args[0])
	if err != nil {
		t.Fatalf("read test binary: %v", err)
	}
	if err := os.WriteFile(program, data, 0o700); err != nil {
		t.Fatalf("write test helper: %v", err)
	}
	return program
}

func rdpTestPayload(passwordLength int) []byte {
	payload := make([]byte, 0, passwordLength+32)
	payload = append(payload, "/v:host\n/p:"...)
	payload = append(payload, bytes.Repeat([]byte{'x'}, passwordLength)...)
	payload = append(payload, '\n')
	return payload
}

func readHelperPID(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		value, err := os.ReadFile(path)
		if err == nil {
			pid, parseErr := strconv.Atoi(strings.TrimSpace(string(value)))
			if parseErr != nil {
				t.Fatalf("parse helper PID: %v", parseErr)
			}
			return pid
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("helper PID file %q was not created", path)
	return 0
}

func waitForPIDExit(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		err := syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("descendant PID %d survived process-group cleanup", pid)
}
