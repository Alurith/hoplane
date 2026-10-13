package askpass

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Alurith/hoplane/internal/secrets"
)

func TestRunAcceptsOnlyOpenSSHPasswordPrompt(t *testing.T) {
	var output bytes.Buffer
	if err := run("alice@example.com's password:", strings.NewReader("secret-value"), &output); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if output.String() != "secret-value\n" {
		t.Fatalf("output = %q", output.String())
	}
}

func TestIsPasswordPromptAcceptsOpenSSHGrammar(t *testing.T) {
	for _, prompt := range []string{
		"alice@example.com's password:",
		"alice@example.com's password: ",
		"alice@example.org@desktop.example.com's password:",
		"alice@example.org@desktop.example.com's password: ",
		"alice@192.0.2.10's password:",
		"alice@[2001:db8::10]'s password:",
		"DOMAIN\\alice@desktop.example.com's password:",
	} {
		if !isPasswordPrompt(prompt) {
			t.Errorf("isPasswordPrompt(%q) = false, want true", prompt)
		}
	}
}

func TestIsPasswordPromptRejectsAmbiguousPrompt(t *testing.T) {
	for _, prompt := range []string{
		"Password:",
		"password:",
		"Enter OTP:",
		"Enter passphrase for key:",
		"alice@example.com's passphrase:",
		"alice@example.com's password: extra",
		" alice@example.com's password:",
		"alice@example.com's password:  ",
		"alice@example.com's password:\t",
		"@example.com's password:",
		"alice@'s password:",
		"alice@example.com's password:\n",
		"alice@example.com's password:\r",
		"alice@example.com's password:\x00",
	} {
		if isPasswordPrompt(prompt) {
			t.Errorf("isPasswordPrompt(%q) = true, want false", prompt)
		}
	}
}

func TestRunRejectsSecretOutsideSSHBoundary(t *testing.T) {
	for _, test := range []struct {
		name  string
		value []byte
	}{
		{name: "empty", value: nil},
		{name: "1024 bytes", value: bytes.Repeat([]byte{'a'}, secrets.MaxSSHAskpassBytes+1)},
		{name: "NUL", value: []byte{'a', 0}},
		{name: "CR", value: []byte("a\r")},
		{name: "LF", value: []byte("a\n")},
		{name: "over sentinel", value: bytes.Repeat([]byte{'a'}, secrets.MaxSSHAskpassBytes+2)},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			if err := run("alice@example.com's password:", bytes.NewReader(test.value), &output); err == nil {
				t.Fatal("run() = nil, want rejection")
			}
			if output.Len() != 0 {
				t.Fatalf("output = %q, want empty", output.String())
			}
		})
	}
}

func TestRunAcceptsSSHBoundary(t *testing.T) {
	for _, length := range []int{1, secrets.MaxSSHAskpassBytes} {
		value := bytes.Repeat([]byte{'a'}, length)
		var output bytes.Buffer
		if err := run("alice@example.com's password: ", bytes.NewReader(value), &output); err != nil {
			t.Fatalf("run(%d bytes) error = %v", length, err)
		}
		if len(output.Bytes()) != length+1 || output.Bytes()[len(output.Bytes())-1] != '\n' {
			t.Fatalf("output length = %d for %d-byte secret", output.Len(), length)
		}
	}
}

func TestRunPropagatesReaderErrorWithoutWriting(t *testing.T) {
	var output bytes.Buffer
	reader := failingReader{}
	if err := run("alice@example.com's password:", reader, &output); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("run() error = %v, want reader error", err)
	}
	if output.Len() != 0 {
		t.Fatalf("output = %q, want empty", output.String())
	}
}

func TestRunConsumesSecretFileOnce(t *testing.T) {
	path, output := askpassFiles(t)
	if err := Run([]string{"alice@example.com's password:"}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("secret still exists: %v", err)
	}
	if err := Run([]string{"alice@example.com's password:"}); err == nil {
		t.Error("second Run() succeeded")
	}
	if _, err := output.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(output)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "test-secret\n" {
		t.Errorf("response = %q, want one response", got)
	}
}

func TestRunUnsupportedPromptPreservesSecretFile(t *testing.T) {
	path, output := askpassFiles(t)
	if err := Run([]string{"Enter OTP:"}); err == nil {
		t.Fatal("Run() accepted an OTP prompt")
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "test-secret" {
		t.Fatalf("secret changed: %v", err)
	}
	info, err := output.Stat()
	if err != nil || info.Size() != 0 {
		t.Fatalf("unsupported prompt produced output: %v", err)
	}
}

func askpassFiles(t *testing.T) (string, *os.File) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "secret")
	if err := os.WriteFile(path, []byte("test-secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(askpassSecretFileEnv, path)
	output, err := os.Create(filepath.Join(dir, "output"))
	if err != nil {
		t.Fatal(err)
	}
	original := os.Stdout
	os.Stdout = output
	t.Cleanup(func() {
		os.Stdout = original
		_ = output.Close()
	})
	return path, output
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }
