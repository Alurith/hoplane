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

func TestRDPOutputPreservesLogicalClientIDsWithoutHardcodedFiltering(t *testing.T) {
	for _, client := range []string{"xfreerdp3", "future-client"} {
		t.Run(client, func(t *testing.T) {
			result := FromDomain(domain.Connection{
				Endpoint: domain.Endpoint{Protocol: domain.ProtocolRDP},
				Options:  domain.Options{"rdp": {"client": client}},
			})
			if got := result.Options["rdp"]["client"]; got != client {
				t.Fatalf("client = %q, want %q", got, client)
			}
		})
	}
}

func TestRDPOutputDoesNotExposeInvalidExecutableLikeClient(t *testing.T) {
	result := FromDomain(domain.Connection{
		Endpoint: domain.Endpoint{Protocol: domain.ProtocolRDP},
		Options:  domain.Options{"rdp": {"client": "/tmp/rdp-client"}},
	})
	if result.Options != nil {
		t.Fatalf("options = %#v, want invalid options omitted", result.Options)
	}
	if result.Security == nil || result.Security.CertificateValidation != "invalid-config" {
		t.Fatalf("security = %#v, want invalid-config", result.Security)
	}
}

func TestWriteList(t *testing.T) {
	var output bytes.Buffer
	err := WriteList(&output, []domain.Connection{{
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

	var object map[string]json.RawMessage
	if err := json.Unmarshal(output.Bytes(), &object); err != nil {
		t.Fatalf("invalid JSON object: %v", err)
	}
	if _, exists := object["warnings"]; exists {
		t.Fatal("list JSON still exposes obsolete warning machinery")
	}
}
