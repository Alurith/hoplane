//go:build linux

package safeio

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestReadFileRejectsFIFOAndOversize(t *testing.T) {
	directory := t.TempDir()
	fifo := filepath.Join(directory, "input")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatalf("Mkfifo() error = %v", err)
	}
	if _, err := ReadFile(fifo, Policy{MaxBytes: 1024, RequireOwner: true, RequirePrivate: true}); !errors.Is(err, ErrNotRegular) {
		t.Fatalf("ReadFile(FIFO) error = %v, want ErrNotRegular", err)
	}

	large := filepath.Join(directory, "large")
	if err := os.WriteFile(large, []byte(strings.Repeat("x", 1025)), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if _, err := ReadFile(large, Policy{MaxBytes: 1024, RequireOwner: true, RequirePrivate: true}); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("ReadFile(large) error = %v, want ErrTooLarge", err)
	}
}

func TestValidateRejectsSymlink(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "target")
	link := filepath.Join(directory, "link")
	if err := os.WriteFile(target, []byte("safe"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("Symlink() error = %v", err)
	}
	if err := Validate(link, Policy{RequireOwner: true, RequirePrivate: true}); !errors.Is(err, ErrNotRegular) {
		t.Fatalf("Validate(symlink) error = %v, want ErrNotRegular", err)
	}
}
