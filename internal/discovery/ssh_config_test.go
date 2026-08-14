package discovery

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Alurith/hoplane/internal/domain"
	"github.com/Alurith/hoplane/internal/sshoptions"
)

func TestSSHConfigSourceDiscoversConcreteAliases(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	contents := `# aliases
Host *
    ServerAliveInterval 30

Host nas bastion !ignored
    HostName internal.example.com

Host NAS
    User duplicate

Host web? wildcard
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	candidates, err := NewSSHConfigSource(path).Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(candidates) != 3 {
		t.Fatalf("candidates = %#v, want three concrete aliases", candidates)
	}
	if got := []string{candidates[0].Name, candidates[1].Name, candidates[2].Name}; !reflect.DeepEqual(got, []string{"nas", "bastion", "wildcard"}) {
		t.Fatalf("aliases = %#v", got)
	}
	if candidates[0].Source.Name != "ssh-config" || candidates[0].Source.ID == "" {
		t.Fatalf("source = %#v", candidates[0].Source)
	}
	if candidates[0].Options[sshoptions.Namespace][sshoptions.HostAlias] != "nas" {
		t.Fatalf("options = %#v", candidates[0].Options)
	}
}

func TestSSHConfigSourceSkipsAliasesUnsafeAsSSHTargets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte("Host user@server -bastion safe\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	candidates, err := NewSSHConfigSource(path).Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(candidates) != 1 || candidates[0].Name != "safe" {
		t.Fatalf("candidates = %#v, want only safe alias", candidates)
	}
}

func TestSSHConfigSourceDiscoversIncludes(t *testing.T) {
	directory := t.TempDir()
	included := filepath.Join(directory, "conf.d", "work.conf")
	if err := os.MkdirAll(filepath.Dir(included), 0o700); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(included, []byte("Host work\n    HostName work.internal\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() included error = %v", err)
	}
	main := filepath.Join(directory, "config")
	if err := os.WriteFile(main, []byte("Include conf.d/*.conf\nHost home\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() main error = %v", err)
	}

	candidates, err := NewSSHConfigSource(main).Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if got := []string{candidates[0].Name, candidates[1].Name}; !reflect.DeepEqual(got, []string{"work", "home"}) {
		t.Fatalf("aliases = %#v", got)
	}
}

func TestSSHConfigSourceMissingFileIsEmpty(t *testing.T) {
	candidates, err := NewSSHConfigSource(filepath.Join(t.TempDir(), "missing")).Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if candidates == nil || len(candidates) != 0 {
		t.Fatalf("candidates = %#v, want empty non-nil slice", candidates)
	}
}

func TestSSHConfigSourceReportsMalformedConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte(`Host "unterminated`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	_, err := NewSSHConfigSource(path).Discover(context.Background())
	if err == nil {
		t.Fatal("Discover() error = nil, want parse error")
	}
}

func TestSSHConfigSourceHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := NewSSHConfigSource(filepath.Join(t.TempDir(), "config")).Discover(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Discover() error = %v, want context.Canceled", err)
	}
}

func TestSSHConfigSourceCandidateNormalizesAsSSH(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte("Host nas\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	candidates, err := NewSSHConfigSource(path).Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	connection, err := domain.NormalizeCandidate(candidates[0])
	if err != nil {
		t.Fatalf("NormalizeCandidate() error = %v", err)
	}
	if connection.Endpoint.Protocol != domain.ProtocolSSH || connection.Endpoint.Port != 22 {
		t.Fatalf("endpoint = %#v", connection.Endpoint)
	}
}
