package askpass

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Alurith/hoplane/internal/secrets"
)

const (
	askpassSecretFileEnv = "HOPLANE_ASKPASS_FILE"
	maxPromptBytes       = 256
)

// Run serves exactly one OpenSSH password prompt. OpenSSH closes inherited
// file descriptors before launching SSH_ASKPASS, so the connector provides a
// short-lived private file instead of relying on FD 3. Keyboard-interactive
// and passphrase prompts are deliberately refused by the caller's SSH options
// and by the strict local prompt check.
func Run(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("askpass requires exactly one prompt")
	}
	path := os.Getenv(askpassSecretFileEnv)
	if path == "" {
		return fmt.Errorf("askpass secret file is unavailable")
	}
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open askpass secret: %w", err)
	}
	defer file.Close()
	return run(args[0], file, os.Stdout)
}

func run(prompt string, reader io.Reader, writer io.Writer) error {
	if !isPasswordPrompt(prompt) {
		return fmt.Errorf("unsupported askpass prompt")
	}
	secret, err := io.ReadAll(io.LimitReader(reader, secrets.MaxSSHAskpassBytes+1))
	if err != nil {
		secrets.Clear(secret)
		return fmt.Errorf("read askpass secret: %w", err)
	}
	defer secrets.Clear(secret)
	if err := secrets.ValidateSecretForProtocol(secrets.ProtocolSSH, secret); err != nil {
		return fmt.Errorf("validate askpass secret: %w", err)
	}
	response := make([]byte, 0, len(secret)+1)
	response = append(response, secret...)
	response = append(response, '\n')
	defer secrets.Clear(response)
	if _, err := writer.Write(response); err != nil {
		return fmt.Errorf("write askpass response: %w", err)
	}
	return nil
}

func isPasswordPrompt(prompt string) bool {
	if prompt == "" || len(prompt) > maxPromptBytes || !utf8.ValidString(prompt) {
		return false
	}
	if strings.HasSuffix(prompt, " ") {
		prompt = prompt[:len(prompt)-1]
	}
	if prompt == "" || strings.HasSuffix(prompt, " ") {
		return false
	}
	const suffix = "'s password:"
	if !strings.HasSuffix(prompt, suffix) {
		return false
	}
	prefix := prompt[:len(prompt)-len(suffix)]
	if strings.Count(prefix, "@") != 1 {
		return false
	}
	at := strings.IndexByte(prefix, '@')
	return validPromptToken(prefix[:at]) && validPromptToken(prefix[at+1:])
}

func validPromptToken(token string) bool {
	if token == "" {
		return false
	}
	for _, r := range token {
		if r == '@' || r == '\'' || unicode.IsSpace(r) || !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}
