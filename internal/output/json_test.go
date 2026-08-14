package output

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/Alurith/hoplane/internal/catalog"
	"github.com/Alurith/hoplane/internal/domain"
)

func TestWriteList(t *testing.T) {
	var output bytes.Buffer
	err := WriteList(&output, catalog.Catalog{Connections: []domain.Connection{{
		Name: "nas",
		Endpoint: domain.Endpoint{
			Protocol: domain.ProtocolSSH,
			Host:     "nas.local",
			Port:     22,
		},
	}}})
	if err != nil {
		t.Fatalf("WriteList() error = %v", err)
	}

	var decoded ListResponse
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if decoded.Version != 1 || len(decoded.Connections) != 1 || decoded.Connections[0].Protocol != "ssh" {
		t.Fatalf("decoded = %#v", decoded)
	}
}
