//go:build linux

package secrets

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

func TestSecretServiceUnavailableIsNotItemAbsence(t *testing.T) {
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path=/tmp/hoplane-no-session-bus")
	_, err := (&secretServiceStore{}).Lookup(context.Background(), Ref{
		ConnectionID: "0123456789abcdef0123456789abcdef",
		Protocol:     ProtocolSSH,
		Kind:         LoginPassword,
	})
	if err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("Lookup() error = %v, want unavailable backend error", err)
	}
}

func TestPresenceWriterScrubsSearchOutput(t *testing.T) {
	value := []byte("keyring-value")
	var writer presenceWriter
	if written, err := writer.Write(value); err != nil || written != len(value) {
		t.Fatalf("Write() = %d, %v", written, err)
	}
	if !writer.present {
		t.Fatal("presence writer did not record output")
	}
	if !bytes.Equal(value, make([]byte, len(value))) {
		t.Fatalf("search output was not scrubbed: %q", value)
	}
}

func TestLimitedWriterClearZerosEveryWrittenByte(t *testing.T) {
	var writer limitedWriter
	writer.limit = 4
	if _, err := writer.Write([]byte("ab")); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("cd")); err != nil {
		t.Fatal(err)
	}
	view := writer.Buffer.Bytes()
	if !bytes.Equal(view, []byte("abcd")) {
		t.Fatalf("written bytes = %q", view)
	}
	if _, err := writer.Write([]byte("overflow")); err != nil {
		t.Fatal(err)
	}
	if !writer.exceeded {
		t.Fatal("overflow was not recorded")
	}
	writer.Clear()
	if len(writer.Bytes()) != 0 || writer.exceeded {
		t.Fatalf("writer after Clear() = len %d, exceeded %v", writer.Len(), writer.exceeded)
	}
	for index, value := range view {
		if value != 0 {
			t.Fatalf("backing byte %d = %d, want zero", index, value)
		}
	}
	if _, err := writer.Write([]byte("xy")); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(writer.Bytes(), []byte("xy")) {
		t.Fatalf("reused writer = %q", writer.Bytes())
	}
}

func TestLimitedWriterExactLimitAndWriteAfterOverflow(t *testing.T) {
	var writer limitedWriter
	writer.limit = 3
	if _, err := writer.Write([]byte("abc")); err != nil {
		t.Fatal(err)
	}
	if writer.exceeded || !bytes.Equal(writer.Bytes(), []byte("abc")) {
		t.Fatalf("exact-limit writer = %q, exceeded %v", writer.Bytes(), writer.exceeded)
	}
	if written, err := writer.Write([]byte("d")); err != nil || written != 1 {
		t.Fatalf("overflow Write() = %d, %v", written, err)
	}
	if !writer.exceeded || !bytes.Equal(writer.Bytes(), []byte("abc")) {
		t.Fatalf("overflow writer = %q, exceeded %v", writer.Bytes(), writer.exceeded)
	}
	writer.Clear()
	if _, err := writer.Write([]byte("z")); err != nil || !bytes.Equal(writer.Bytes(), []byte("z")) {
		t.Fatalf("writer reuse = %q, %v", writer.Bytes(), err)
	}
}
