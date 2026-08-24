package connector

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Alurith/hoplane/internal/safeio"
	"github.com/Alurith/hoplane/internal/secrets"
)

// ProcessRunner abstracts process lookup and execution for connector tests.
type ProcessRunner interface {
	LookPath(string) (string, error)
	Run(context.Context, Invocation, IO) error
}

type SecretTransport uint8

const (
	SecretTransportRDPArgsFD SecretTransport = iota + 1
	SecretTransportSSHAskpass
)

type SecretProcessRequest struct {
	Transport SecretTransport
	// Payload is transferred to RunWithSecret. The caller must not read or
	// clear it after the call; RunWithSecret owns and clears it.
	Payload []byte
}

type SecretProcessRunner interface {
	ProcessRunner
	RunWithSecret(context.Context, Invocation, IO, SecretProcessRequest) error
}

// ExecRunner runs processes using the host operating system.
type ExecRunner struct{}

func (ExecRunner) LookPath(name string) (string, error) {
	if err := validateExecutableName(name); err != nil {
		return "", err
	}
	path, err := exec.LookPath(name)
	if err != nil {
		return "", err
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("resolve executable %q: %w", name, err)
	}
	if err := validateExecutable(path); err != nil {
		return "", fmt.Errorf("validate executable %q: %w", path, err)
	}
	return path, nil
}

func (ExecRunner) Run(ctx context.Context, invocation Invocation, streams IO) error {
	command, err := newCommand(ctx, invocation, streams)
	if err != nil {
		return err
	}
	return command.Run()
}

func validateExecutableName(name string) error {
	if name == "" || name == "." || name == ".." || filepath.IsAbs(name) ||
		filepath.Base(name) != name || strings.ContainsAny(name, `/\\`) {
		return fmt.Errorf("executable %q must be a plain program name", name)
	}
	for index, r := range name {
		letter := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
		digit := r >= '0' && r <= '9'
		if letter || digit || index > 0 && strings.ContainsRune("._+-", r) {
			continue
		}
		return fmt.Errorf("executable %q must be a plain program name", name)
	}
	return nil
}

const (
	secureCommandWaitDelay = 2 * time.Second
	secureTermGrace        = 100 * time.Millisecond
	secureKillWait         = time.Second
)

func (ExecRunner) RunWithSecret(
	ctx context.Context,
	invocation Invocation,
	streams IO,
	request SecretProcessRequest,
) (result error) {
	payload := request.Payload
	// RunWithSecret is the sole owner from this point, including validation and
	// every setup failure. The writer goroutine never clears payload.
	defer secrets.Clear(payload)

	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateSecretProcessRequest(request); err != nil {
		return err
	}

	var (
		reader             *os.File
		writer             *os.File
		askpassSecretPath  string
		cleanupAskpassFile func() error
		err                error
	)
	defer func() {
		if cleanupAskpassFile == nil {
			return
		}
		if err := cleanupAskpassFile(); err != nil {
			result = errors.Join(result, fmt.Errorf("remove askpass secret: %w", err))
		}
	}()
	switch request.Transport {
	case SecretTransportSSHAskpass:
		askpassSecretPath, cleanupAskpassFile, err = prepareAskpassSecret(payload)
		if err != nil {
			return err
		}
		// Cleanup is handled by the deferred error-reporting cleanup above.
	case SecretTransportRDPArgsFD:
		reader, writer, err = os.Pipe()
		if err != nil {
			return fmt.Errorf("create secret pipe: %w", err)
		}
		defer reader.Close()
	default:
		return fmt.Errorf("unsupported secret transport %d", request.Transport)
	}

	var closeWriterOnce sync.Once
	var closeWriterErr error
	closeWriter := func() error {
		closeWriterOnce.Do(func() {
			if writer != nil {
				closeWriterErr = writer.Close()
			}
		})
		return closeWriterErr
	}
	defer closeWriter()

	command, err := newCommand(ctx, invocation, streams)
	if err != nil {
		return err
	}
	if reader != nil {
		command.ExtraFiles = []*os.File{reader}
	}
	restoreTerminal, err := configureSecureCommand(command, streams.Input)
	if err != nil {
		return fmt.Errorf("configure secure process: %w", err)
	}
	switch request.Transport {
	case SecretTransportSSHAskpass:
		if err := configureAskpass(command, askpassSecretPath); err != nil {
			return err
		}
	case SecretTransportRDPArgsFD:
		// The payload validator already checked the RDP framing.
	}
	command.WaitDelay = secureCommandWaitDelay

	var stopErr error
	var stopErrMu sync.Mutex
	var stopRequested atomic.Bool
	var processStarted atomic.Bool
	var processDone atomic.Bool
	var processGroupID int
	var processGroupIDMu sync.Mutex
	var cleanupOnce sync.Once
	stopSecureProcess := func() error {
		stopRequested.Store(true)
		// Closing the parent writer first prevents a blocked write from
		// outliving process-group cleanup. This is also safe before Start.
		closeErr := closeWriter()
		processGroupIDMu.Lock()
		groupID := processGroupID
		started := processStarted.Load()
		processGroupIDMu.Unlock()
		if started {
			cleanupOnce.Do(func() {
				if closeErr != nil {
					stopErrMu.Lock()
					stopErr = fmt.Errorf("close secret pipe writer: %w", closeErr)
					stopErrMu.Unlock()
				}
				if err := terminateSecureCommandWithEscalation(command, groupID, &processDone, secureTermGrace, secureKillWait); err != nil {
					stopErrMu.Lock()
					if stopErr == nil {
						stopErr = err
					}
					stopErrMu.Unlock()
				}
			})
		}
		stopErrMu.Lock()
		err := stopErr
		stopErrMu.Unlock()
		return err
	}
	command.Cancel = stopSecureProcess

	if err := ctx.Err(); err != nil {
		return err
	}
	if err := command.Start(); err != nil {
		return fmt.Errorf("start secure process: %w", err)
	}
	processGroupIDMu.Lock()
	processGroupID = secureProcessGroupID(command)
	processStarted.Store(true)
	pendingStop := stopRequested.Load()
	processGroupIDMu.Unlock()
	if pendingStop {
		_ = stopSecureProcess()
	}

	waitDone := make(chan error, 1)
	go func() {
		err := command.Wait()
		processDone.Store(true)
		waitDone <- err
	}()

	var readerErr error
	if reader != nil {
		readerErr = reader.Close()
	}
	writeDone := make(chan error, 1)
	if writer == nil {
		writeDone <- nil
	} else {
		go func() {
			_, writeErr := io.Copy(writer, bytes.NewReader(payload))
			if closeErr := closeWriter(); writeErr == nil {
				writeErr = closeErr
			}
			writeDone <- writeErr
		}()
	}

	if readerErr != nil {
		_ = stopSecureProcess()
	}

	var (
		waitErr            error
		writeErr           error
		writeErrArtificial bool
		writeErrReal       bool
		waitSeen           bool
		writeSeen          bool
		ctxDone            = ctx.Done()
	)
	for !waitSeen || !writeSeen {
		select {
		case err := <-waitDone:
			waitErr = err
			waitSeen = true
			_ = stopSecureProcess()
		case err := <-writeDone:
			writeErr = err
			writeSeen = true
			if err != nil {
				writeErrArtificial = isArtificialSecretWriteError(err) || stopRequested.Load() || processDone.Load()
				writeErrReal = !writeErrArtificial
				_ = stopSecureProcess()
			}
		case <-ctxDone:
			ctxDone = nil
			_ = stopSecureProcess()
		}
	}

	stopErrMu.Lock()
	cleanupErr := stopErr
	stopErrMu.Unlock()

	var runErr error
	if ctxErr := ctx.Err(); ctxErr != nil {
		runErr = ctxErr
		if waitErr != nil {
			runErr = errors.Join(runErr, waitErr)
		}
		if writeErrReal {
			runErr = errors.Join(runErr, fmt.Errorf("write secret transport: %w", writeErr))
		}
	} else {
		switch {
		case readerErr != nil:
			runErr = fmt.Errorf("close secret pipe reader: %w", readerErr)
		case writeErrReal:
			runErr = fmt.Errorf("write secret transport: %w", writeErr)
			if waitErr != nil {
				runErr = errors.Join(runErr, waitErr)
			}
		case waitErr != nil:
			runErr = waitErr
		case writeErr != nil && !writeErrArtificial:
			runErr = fmt.Errorf("write secret transport: %w", writeErr)
		}
	}
	if err := restoreTerminal(); err != nil {
		runErr = errors.Join(runErr, fmt.Errorf("restore terminal process group: %w", err))
	}
	if cleanupErr != nil {
		if runErr == nil {
			return cleanupErr
		}
		return errors.Join(runErr, cleanupErr)
	}
	return runErr
}

func validateSecretProcessRequest(request SecretProcessRequest) error {
	switch request.Transport {
	case SecretTransportSSHAskpass:
		return secrets.ValidateSecretForProtocol(secrets.ProtocolSSH, request.Payload)
	case SecretTransportRDPArgsFD:
		return validateRDPArgsPayload(request.Payload)
	default:
		return fmt.Errorf("unsupported secret transport %d", request.Transport)
	}
}

func terminateSecureCommandWithEscalation(command *exec.Cmd, processGroupID int, processDone *atomic.Bool, termGrace, killWait time.Duration) error {
	if err := terminateSecureCommand(command, processGroupID); err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			return nil
		}
		return fmt.Errorf("terminate secure process: %w", err)
	}
	if !waitForSecureProcessGroup(command, processGroupID, processDone, termGrace) {
		return nil
	}
	if err := killSecureCommand(command, processGroupID); err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			return nil
		}
		return fmt.Errorf("kill secure process group: %w", err)
	}
	if waitForSecureProcessGroup(command, processGroupID, processDone, killWait) {
		return errors.New("secure process group did not terminate")
	}
	return nil
}

func waitForSecureProcessGroup(command *exec.Cmd, processGroupID int, processDone *atomic.Bool, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for secureProcessGroupAlive(command, processGroupID, processDone) {
		if time.Now().After(deadline) {
			return true
		}
		time.Sleep(5 * time.Millisecond)
	}
	return false
}

func newCommand(ctx context.Context, invocation Invocation, streams IO) (*exec.Cmd, error) {
	if !filepath.IsAbs(invocation.Program) {
		return nil, fmt.Errorf("executable path must be absolute")
	}
	if err := validateExecutable(invocation.Program); err != nil {
		return nil, fmt.Errorf("validate executable %q: %w", invocation.Program, err)
	}
	command := exec.CommandContext(ctx, invocation.Program, invocation.Args...)
	command.Stdin = streams.Input
	command.Stdout = streams.Output
	command.Stderr = streams.Errors
	return command, nil
}

func prepareAskpassSecret(payload []byte) (string, func() error, error) {
	directory, err := os.MkdirTemp("", "hoplane-askpass-")
	if err != nil {
		return "", nil, fmt.Errorf("create askpass directory: %w", err)
	}
	cleanup := func() error { return os.RemoveAll(directory) }
	fail := func(operation string, cause error) (string, func() error, error) {
		if cleanupErr := cleanup(); cleanupErr != nil {
			cause = errors.Join(cause, fmt.Errorf("remove askpass directory: %w", cleanupErr))
		}
		return "", nil, fmt.Errorf("%s: %w", operation, cause)
	}
	if err := safeio.ValidateAncestors(directory); err != nil {
		return fail("validate askpass directory", err)
	}
	if err := safeio.ValidateDirectory(directory, safeio.Policy{
		RequireOwner:          true,
		RejectGroupOtherWrite: true,
	}); err != nil {
		return fail("validate askpass directory", err)
	}
	path := filepath.Join(directory, "secret")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fail("create askpass secret", err)
	}
	written, writeErr := io.Copy(file, bytes.NewReader(payload))
	if writeErr == nil && written != int64(len(payload)) {
		writeErr = io.ErrShortWrite
	}
	closeErr := file.Close()
	if writeErr != nil {
		return fail("write askpass secret", writeErr)
	}
	if closeErr != nil {
		return fail("close askpass secret", closeErr)
	}
	return path, cleanup, nil
}

func configureAskpass(command *exec.Cmd, secretPath string) error {
	if secretPath == "" {
		return errors.New("askpass secret path is empty")
	}
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve Hoplane askpass executable: %w", err)
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return fmt.Errorf("resolve Hoplane askpass path: %w", err)
	}
	if err := validateExecutable(executable); err != nil {
		return fmt.Errorf("validate Hoplane askpass executable: %w", err)
	}
	command.Env = setEnvironment(command.Env, map[string]string{
		"SSH_ASKPASS":          executable,
		"SSH_ASKPASS_REQUIRE":  "force",
		"HOPLANE_ASKPASS":      "1",
		"HOPLANE_ASKPASS_FILE": secretPath,
		"DISPLAY":              "hoplane",
	})
	return nil
}

func setEnvironment(environment []string, values map[string]string) []string {
	if environment == nil {
		environment = os.Environ()
	}
	result := make([]string, 0, len(environment)+len(values))
	for _, entry := range environment {
		name, _, ok := strings.Cut(entry, "=")
		if _, replace := values[name]; ok && replace {
			continue
		}
		result = append(result, entry)
	}
	for name, value := range values {
		result = append(result, name+"="+value)
	}
	return result
}
