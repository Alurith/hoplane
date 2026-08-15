package output

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/Alurith/hoplane/internal/catalog"
	"github.com/Alurith/hoplane/internal/domain"
	"github.com/Alurith/hoplane/internal/sshoptions"
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

func TestSSHOutputUsesTypedConfigReference(t *testing.T) {
	connection := domain.Connection{
		Endpoint: domain.Endpoint{
			Protocol: domain.ProtocolSSH,
			Host:     "nas",
			Port:     22,
		},
		Metadata: domain.Metadata{sshoptions.Namespace: {
			sshoptions.ConfigFile: "/home/alice/.ssh/config",
			sshoptions.HostAlias:  "nas",
		}},
	}
	result := FromDomain(connection)
	if result.SSHConfig == nil || result.SSHConfig.File == "" || result.SSHConfig.Alias != "nas" {
		t.Fatalf("SSHConfig = %#v, want typed reference", result.SSHConfig)
	}
}

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
