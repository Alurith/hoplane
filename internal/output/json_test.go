package output

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/Alurith/hoplane/internal/domain"
)

func TestRDPOutputReportsEffectiveCertificatePosture(t *testing.T) {
	connection := domain.Connection{
		Endpoint: domain.Endpoint{
			Protocol: domain.ProtocolRDP,
			Host:     "desktop.example.com",
			Port:     3389,
		},
		Options: domain.Options{"rdp": {"ignore_certificate": "TRUE"}},
	}
	result := FromDomain(connection)
	if result.Security == nil || result.Security.CertificateValidation != "ignored" {
		t.Fatalf("security = %#v, want ignored", result.Security)
	}
}

func TestWriteList(t *testing.T) {
	var output bytes.Buffer
	err := WriteList(&output, []domain.Connection{{
		ID:   "0123456789abcdef0123456789abcdef",
		Name: "nas",
		Endpoint: domain.Endpoint{
			Protocol: domain.ProtocolSSH,
			Host:     "nas.local",
			Port:     22,
		},
	}})
	if err != nil {
		t.Fatalf("WriteList() error = %v", err)
	}

	var decoded ListResponse
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if decoded.Version != 2 || len(decoded.Connections) != 1 || decoded.Connections[0].Protocol != "ssh" {
		t.Fatalf("decoded = %#v", decoded)
	}
	if decoded.Connections[0].ID != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("connection ID = %q", decoded.Connections[0].ID)
	}
	if bytes.Contains(output.Bytes(), []byte("password")) || bytes.Contains(output.Bytes(), []byte("secret")) {
		t.Fatalf("JSON contains secret-related value: %s", output.Bytes())
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(output.Bytes(), &object); err != nil {
		t.Fatalf("invalid JSON object: %v", err)
	}
	if _, exists := object["warnings"]; exists {
		t.Fatal("list JSON still exposes obsolete warning machinery")
	}
}
