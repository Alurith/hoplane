package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/Alurith/hoplane/internal/output"
)

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
