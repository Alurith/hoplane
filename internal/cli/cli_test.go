package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Alurith/hoplane/internal/config"
	"github.com/Alurith/hoplane/internal/output"
	"github.com/Alurith/hoplane/internal/secrets"
)

func TestReadSecretAcceptsMaxSizedCRLFLine(t *testing.T) {
	input := append(bytes.Repeat([]byte{'a'}, secrets.MaxSSHAskpassBytes), '\r', '\n')
	value, err := readSecret(bytes.NewReader(input), &bytes.Buffer{}, secrets.ProtocolSSH)
	if err != nil {
		t.Fatalf("readSecret() error = %v", err)
	}
	defer secrets.Clear(value)
	if len(value) != secrets.MaxSSHAskpassBytes {
		t.Fatalf("readSecret() length = %d, want %d", len(value), secrets.MaxSSHAskpassBytes)
	}
}

func TestReadSecretReturnsAfterNewline(t *testing.T) {
	reader, writer := io.Pipe()
	t.Cleanup(func() { _ = reader.Close(); _ = writer.Close() })
	type result struct {
		value []byte
		err   error
	}
	done := make(chan result, 1)
	go func() {
		value, err := readSecret(reader, io.Discard, secrets.ProtocolSSH)
		done <- result{value, err}
	}()
	if _, err := writer.Write([]byte("password\n")); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-done:
		defer secrets.Clear(got.value)
		if got.err != nil || string(got.value) != "password" {
			t.Fatalf("readSecret() error = %v", got.err)
		}
	case <-time.After(time.Second):
		_ = writer.Close()
		got := <-done
		secrets.Clear(got.value)
		t.Fatal("readSecret() waited for EOF after a complete line")
	}
}

func TestReadSecretBoundaries(t *testing.T) {
	for _, test := range []struct {
		protocol string
		max      int
	}{
		{secrets.ProtocolSSH, 1023}, {secrets.ProtocolRDP, 1 << 20},
	} {
		t.Run(test.protocol, func(t *testing.T) {
			for _, terminator := range []string{"", "\n", "\r\n"} {
				password := strings.Repeat("a", test.max)
				value, err := readSecret(strings.NewReader(password+terminator), io.Discard, test.protocol)
				if err != nil || string(value) != password {
					t.Errorf("maximum password with terminator %q: %v", terminator, err)
				}
				secrets.Clear(value)
				value, err = readSecret(strings.NewReader(password+"a"+terminator), io.Discard, test.protocol)
				if err == nil || len(value) != 0 {
					t.Errorf("oversized password with terminator %q accepted", terminator)
				}
				secrets.Clear(value)
			}
			for _, input := range []string{"", "\n", "\r\n", "a\x00b\n", "a\rb\n", "password\r"} {
				value, err := readSecret(strings.NewReader(input), io.Discard, test.protocol)
				secrets.Clear(value)
				if err == nil || len(value) != 0 {
					t.Errorf("invalid input %q accepted", input)
				}
			}
		})
	}
	value, err := readSecret(strings.NewReader("first\nsecond\n"), io.Discard, secrets.ProtocolSSH)
	defer secrets.Clear(value)
	if err != nil || string(value) != "first" {
		t.Errorf("did not read only the first line: %v", err)
	}
}

func TestReadSecretPropagatesInputError(t *testing.T) {
	value, err := readSecret(secretErrorReader{}, io.Discard, secrets.ProtocolSSH)
	defer secrets.Clear(value)
	if !errors.Is(err, io.ErrUnexpectedEOF) || len(value) != 0 {
		t.Fatalf("input error lost: %v", err)
	}
}

type secretErrorReader struct{}

func (secretErrorReader) Read(p []byte) (int, error) { p[0] = 'a'; return 1, io.ErrUnexpectedEOF }

func TestSecretCommandsManageSSHPasswordWithoutPrintingIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	store := secrets.NewMemoryStore()
	var outputBuffer bytes.Buffer
	if err := Execute(context.Background(), []string{
		"add", "nas", "--config", path, "--protocol", "ssh", "--host", "nas.local", "--user", "alice",
	}, Dependencies{Input: bytes.NewBuffer(nil), Output: &outputBuffer, Errors: &bytes.Buffer{}, SecretStore: store}); err != nil {
		t.Fatalf("add error = %v", err)
	}
	outputBuffer.Reset()
	if err := Execute(context.Background(), []string{"secret", "set", "nas", "--config", path}, Dependencies{
		Input: bytes.NewBufferString("super-secret\n"), Output: &outputBuffer, Errors: &bytes.Buffer{}, SecretStore: store,
	}); err != nil {
		t.Fatalf("secret set error = %v", err)
	}
	if strings.Contains(outputBuffer.String(), "super-secret") {
		t.Fatal("secret appeared in command output")
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(contents), "super-secret") {
		t.Fatal("secret appeared in configuration")
	}
	outputBuffer.Reset()
	if err := Execute(context.Background(), []string{"secret", "status", "nas", "--config", path}, Dependencies{
		Input: bytes.NewBuffer(nil), Output: &outputBuffer, Errors: &bytes.Buffer{}, SecretStore: store,
	}); err != nil {
		t.Fatalf("secret status error = %v", err)
	}
	if outputBuffer.String() != "nas: configured\n" {
		t.Fatalf("status = %q", outputBuffer.String())
	}
}

func TestAddPersistsStandardSSH(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := Execute(context.Background(), []string{
		"add", "nas", "--config", path,
		"--protocol", "ssh", "--host", "nas.local", "--user", "alice",
	}, Dependencies{
		Input:  bytes.NewBuffer(nil),
		Output: &bytes.Buffer{},
		Errors: &bytes.Buffer{},
	}); err != nil {
		t.Fatalf("add error = %v", err)
	}

	file, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if options := file.Connections[0].Options; len(options) != 0 {
		t.Fatalf("SSH options = %#v, want none", options)
	}
}

func TestAddPersistsRDPOptions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	err := Execute(context.Background(), []string{
		"add", "office", "--config", path,
		"--protocol", "rdp", "--host", "desktop.example.com", "--user", "alice",
		"--rdp-domain", "CONTOSO", "--rdp-fullscreen", "--rdp-ignore-certificate",
	}, Dependencies{
		Input:  bytes.NewBuffer(nil),
		Output: &bytes.Buffer{},
		Errors: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("add error = %v", err)
	}

	file, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	options := file.Connections[0].Options["rdp"]
	want := map[string]string{
		"domain":             "CONTOSO",
		"fullscreen":         "true",
		"ignore_certificate": "true",
	}
	if !reflect.DeepEqual(options, want) {
		t.Fatalf("options = %#v, want %#v", options, want)
	}
}

func TestAddRejectsEmptyRDPDomain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	err := Execute(context.Background(), []string{
		"add", "office", "--config", path,
		"--protocol", "rdp", "--host", "desktop.example.com", "--rdp-domain", " ",
	}, Dependencies{
		Input:  bytes.NewBuffer(nil),
		Output: &bytes.Buffer{},
		Errors: &bytes.Buffer{},
	})
	if err == nil || !strings.Contains(err.Error(), `RDP option "domain" cannot be empty`) {
		t.Fatalf("add error = %v, want empty domain error", err)
	}
}

func TestAddRejectsObsoleteSSHFlag(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	err := Execute(context.Background(), []string{
		"add", "nas", "--config", path,
		"--protocol", "ssh", "--host", "nas.local", "--identity-file", "/tmp/id",
	}, Dependencies{
		Input:  bytes.NewBuffer(nil),
		Output: &bytes.Buffer{},
		Errors: &bytes.Buffer{},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown flag") {
		t.Fatalf("add error = %v, want obsolete flag rejection", err)
	}
}

func TestAddRejectsRDPOptionsForSSH(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")

	err := Execute(context.Background(), []string{
		"add", "nas", "--config", path,
		"--protocol", "ssh", "--host", "nas.local", "--rdp-domain", "CONTOSO",
	}, Dependencies{
		Input:  bytes.NewBuffer(nil),
		Output: &bytes.Buffer{},
		Errors: &bytes.Buffer{},
	})
	if err == nil || !strings.Contains(err.Error(), `RDP options require protocol "rdp"`) {
		t.Fatalf("add error = %v, want protocol validation error", err)
	}
}

func TestAddListAndShow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	var outputBuffer bytes.Buffer
	dependencies := Dependencies{Input: bytes.NewBuffer(nil), Output: &outputBuffer, Errors: &bytes.Buffer{}}

	if err := Execute(context.Background(), []string{
		"add", "office", "--config", path, "--protocol", "rdp", "--host", "desktop.example.com", "--user", "alice",
	}, dependencies); err != nil {
		t.Fatalf("add error = %v", err)
	}
	outputBuffer.Reset()

	if err := Execute(context.Background(), []string{"list", "--config", path}, dependencies); err != nil {
		t.Fatalf("list error = %v", err)
	}
	var listed output.ListResponse
	if err := json.Unmarshal(outputBuffer.Bytes(), &listed); err != nil {
		t.Fatalf("list output is not JSON: %v", err)
	}
	if len(listed.Connections) != 1 || listed.Connections[0].Port != 3389 {
		t.Fatalf("listed = %#v", listed)
	}

	outputBuffer.Reset()
	if err := Execute(context.Background(), []string{"show", "office", "--config", path}, dependencies); err != nil {
		t.Fatalf("show error = %v", err)
	}
	var shown output.ConnectionResponse
	if err := json.Unmarshal(outputBuffer.Bytes(), &shown); err != nil {
		t.Fatalf("show output is not JSON: %v", err)
	}
	if shown.Connection.Name != "office" {
		t.Fatalf("shown = %#v", shown)
	}
}
