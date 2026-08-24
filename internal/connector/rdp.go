package connector

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"

	"github.com/Alurith/hoplane/internal/domain"
	"github.com/Alurith/hoplane/internal/rdpoptions"
	"github.com/Alurith/hoplane/internal/secrets"
)

// RDPConnector selects a registered platform client and executes its plan.
type RDPConnector struct {
	runner        ProcessRunner
	secretStore   secrets.Store
	defaultClient string
	clients       map[string]rdpClient
}

type rdpClient interface {
	ID() string
	Program() string
	Capabilities() rdpClientCapabilities
	Plan(domain.Connection, rdpoptions.Options) ([]string, error)
}

type rdpClientCapabilities struct {
	Fullscreen        bool
	IgnoreCertificate bool
	PasswordTransport SecretTransport
}

func NewRDPConnector(runner ProcessRunner, store ...secrets.Store) RDPConnector {
	defaultClient, clients := platformRDPClients()
	var secretStore secrets.Store
	if len(store) > 0 {
		secretStore = store[0]
	}
	if secretStore == nil {
		secretStore = secrets.NewDefaultStore()
	}
	return newRDPConnector(runner, secretStore, defaultClient, clients)
}

func newRDPConnector(
	runner ProcessRunner,
	secretStore secrets.Store,
	defaultClient string,
	clients map[string]rdpClient,
) RDPConnector {
	if runner == nil {
		runner = ExecRunner{}
	}
	return RDPConnector{
		runner:        runner,
		secretStore:   secretStore,
		defaultClient: defaultClient,
		clients:       clients,
	}
}

func (RDPConnector) Protocol() domain.Protocol {
	return domain.ProtocolRDP
}

func (c RDPConnector) Plan(connection domain.Connection) (Invocation, error) {
	_, invocation, err := c.plan(connection)
	return invocation, err
}

func (c RDPConnector) plan(connection domain.Connection) (string, Invocation, error) {
	if connection.Endpoint.Protocol != c.Protocol() {
		return "", Invocation{}, fmt.Errorf(
			"RDP connector cannot handle protocol %q",
			connection.Endpoint.Protocol,
		)
	}
	if connection.Endpoint.Host == "" {
		return "", Invocation{}, fmt.Errorf("RDP connection host cannot be empty")
	}
	if connection.Endpoint.Port == 0 {
		return "", Invocation{}, fmt.Errorf("RDP connection port cannot be zero")
	}

	options, err := rdpoptions.Decode(connection.Options)
	if err != nil {
		return "", Invocation{}, fmt.Errorf("decode RDP options: %w", err)
	}

	clientID := options.Client
	if clientID == "" {
		clientID = c.defaultClient
	}
	if clientID == "" {
		return "", Invocation{}, fmt.Errorf("no RDP client is registered for platform %q", runtime.GOOS)
	}
	client, ok := c.clients[clientID]
	if !ok || client == nil {
		return "", Invocation{}, fmt.Errorf(
			"RDP client %q is not registered for platform %q",
			clientID,
			runtime.GOOS,
		)
	}
	if client.ID() != clientID {
		return "", Invocation{}, fmt.Errorf(
			"RDP client registration %q has adapter ID %q",
			clientID,
			client.ID(),
		)
	}
	if err := validateRDPClientOptions(clientID, client.Capabilities(), options); err != nil {
		return "", Invocation{}, err
	}
	program := client.Program()
	if err := validateExecutableName(program); err != nil {
		return "", Invocation{}, fmt.Errorf("RDP client %q has invalid executable: %w", clientID, err)
	}
	args, err := client.Plan(connection, options)
	if err != nil {
		return "", Invocation{}, fmt.Errorf("plan with RDP client %q: %w", clientID, err)
	}
	return clientID, Invocation{Program: program, Args: args}, nil
}

func validateRDPClientOptions(
	clientID string,
	capabilities rdpClientCapabilities,
	options rdpoptions.Options,
) error {
	if options.Fullscreen && !capabilities.Fullscreen {
		return fmt.Errorf("RDP client %q does not support option %q", clientID, rdpoptions.Fullscreen)
	}
	if options.IgnoreCertificate && !capabilities.IgnoreCertificate {
		return fmt.Errorf("RDP client %q does not support option %q", clientID, rdpoptions.IgnoreCertificate)
	}
	return nil
}

func (c RDPConnector) Connect(
	ctx context.Context,
	connection domain.Connection,
	streams IO,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	clientID, invocation, err := c.plan(connection)
	if err != nil {
		return err
	}

	programName := invocation.Program
	program, err := c.runner.LookPath(programName)
	if err != nil {
		return fmt.Errorf(
			"%w: executable %q for RDP client %q is not available: %w",
			ErrClientUnavailable,
			programName,
			clientID,
			err,
		)
	}
	invocation.Program = program

	secret, secretErr := c.lookupPassword(ctx, connection)
	if errors.Is(secretErr, secrets.ErrNotFound) {
		secrets.Clear(secret)
		if err := c.runner.Run(ctx, invocation, streams); err != nil {
			return fmt.Errorf("run %s: %w", programName, err)
		}
		return nil
	}
	if secretErr != nil {
		secrets.Clear(secret)
		return secretErr
	}
	if err := secrets.ValidateSecretForProtocol(secrets.ProtocolRDP, secret); err != nil {
		secrets.Clear(secret)
		return err
	}

	client := c.clients[clientID]
	transport := client.Capabilities().PasswordTransport
	if transport != SecretTransportRDPArgsFD {
		secrets.Clear(secret)
		return fmt.Errorf("%w for RDP client %q", ErrSecureTransport, clientID)
	}
	secureRunner, ok := c.runner.(SecretProcessRunner)
	if !ok {
		secrets.Clear(secret)
		return fmt.Errorf("%w for RDP client %q", ErrSecureTransport, clientID)
	}
	if err := validateRDPArgs(invocation.Args); err != nil {
		secrets.Clear(secret)
		return err
	}
	payload := rdpArgsPayload(invocation.Args, secret)
	// rdpArgsPayload copied the password into the sole transport buffer.
	secrets.Clear(secret)
	if err := validateRDPArgsPayload(payload); err != nil {
		secrets.Clear(payload)
		return err
	}
	secureInvocation := Invocation{Program: invocation.Program, Args: []string{"/args-from:fd:3"}}
	if err := secureRunner.RunWithSecret(ctx, secureInvocation, streams, SecretProcessRequest{
		Transport: transport,
		Payload:   payload,
	}); err != nil {
		return fmt.Errorf("run %s with keyring password: %w", programName, err)
	}
	return nil
}

func (c RDPConnector) lookupPassword(ctx context.Context, connection domain.Connection) ([]byte, error) {
	if connection.ID == "" || c.secretStore == nil {
		return nil, secrets.ErrNotFound
	}
	return c.secretStore.Lookup(ctx, secrets.Ref{
		ConnectionID: connection.ID,
		Protocol:     secrets.ProtocolRDP,
		Kind:         secrets.LoginPassword,
	})
}

const maxRDPArgsPayloadOverhead = 64 * 1024

func validateRDPArgs(args []string) error {
	for _, arg := range args {
		if arg == "" || strings.IndexByte(arg, 0) >= 0 || strings.ContainsAny(arg, "\r\n") {
			return fmt.Errorf("RDP argument contains a line delimiter")
		}
	}
	return nil
}

func rdpArgsPayload(args []string, password []byte) []byte {
	var payload bytes.Buffer
	for _, arg := range args {
		payload.WriteString(arg)
		payload.WriteByte('\n')
	}
	payload.WriteString("/p:")
	payload.Write(password)
	payload.WriteByte('\n')
	return payload.Bytes()
}

func validateRDPArgsPayload(payload []byte) error {
	if len(payload) == 0 || len(payload) > secrets.MaxRDPPasswordBytes+maxRDPArgsPayloadOverhead {
		return fmt.Errorf("RDP secret transport payload is invalid")
	}
	if bytes.IndexByte(payload, 0) >= 0 || bytes.IndexByte(payload, '\r') >= 0 {
		return fmt.Errorf("RDP secret transport payload contains an invalid character")
	}
	if payload[len(payload)-1] != '\n' {
		return fmt.Errorf("RDP secret transport payload is not line terminated")
	}
	body := payload[:len(payload)-1]
	lastLine := bytes.LastIndexByte(body, '\n')
	passwordLine := body[lastLine+1:]
	if !bytes.HasPrefix(passwordLine, []byte("/p:")) {
		return fmt.Errorf("RDP secret transport payload has no password argument")
	}
	if err := secrets.ValidateSecretForProtocol(secrets.ProtocolRDP, passwordLine[len("/p:"):]); err != nil {
		return fmt.Errorf("validate RDP secret transport: %w", err)
	}
	if lastLine >= 0 {
		arguments := body[:lastLine]
		if len(arguments) == 0 || arguments[0] == '\n' || arguments[len(arguments)-1] == '\n' || bytes.Contains(arguments, []byte("\n\n")) {
			return fmt.Errorf("RDP secret transport contains an empty argument")
		}
	}
	return nil
}
