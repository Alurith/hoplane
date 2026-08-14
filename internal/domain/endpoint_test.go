package domain

import "testing"

func TestNormalizeCandidate(t *testing.T) {
	port := uint16(3390)
	connection, err := NormalizeCandidate(Candidate{
		Name:        " Office ",
		Protocol:    "RDP",
		Host:        "DESKTOP.EXAMPLE.COM",
		Port:        &port,
		User:        "alice",
		Description: " Work desktop ",
		Tags:        []string{"work", "work", " desktop "},
		Source:      SourceRef{Name: "static"},
	})
	if err != nil {
		t.Fatalf("NormalizeCandidate() error = %v", err)
	}

	if connection.Name != "Office" {
		t.Fatalf("name = %q, want Office", connection.Name)
	}
	if connection.Endpoint.Protocol != ProtocolRDP {
		t.Fatalf("protocol = %q, want rdp", connection.Endpoint.Protocol)
	}
	if connection.Endpoint.Host != "desktop.example.com" {
		t.Fatalf("host = %q, want desktop.example.com", connection.Endpoint.Host)
	}
	if connection.Endpoint.Port != 3390 {
		t.Fatalf("port = %d, want 3390", connection.Endpoint.Port)
	}
	if got := connection.Endpoint.Address(); got != "desktop.example.com:3390" {
		t.Fatalf("address = %q, want desktop.example.com:3390", got)
	}
	if len(connection.Tags) != 2 || connection.Tags[0] != "desktop" || connection.Tags[1] != "work" {
		t.Fatalf("tags = %#v, want [desktop work]", connection.Tags)
	}
}

func TestNormalizeCandidateUsesDefaultPort(t *testing.T) {
	connection, err := NormalizeCandidate(Candidate{
		Name:     "nas",
		Protocol: "ssh",
		Host:     "nas.local",
	})
	if err != nil {
		t.Fatalf("NormalizeCandidate() error = %v", err)
	}
	if connection.Endpoint.Port != 22 {
		t.Fatalf("port = %d, want 22", connection.Endpoint.Port)
	}
}

func TestNormalizeCandidateRequiresPortForUnknownProtocol(t *testing.T) {
	_, err := NormalizeCandidate(Candidate{
		Name:     "custom",
		Protocol: "custom-protocol",
		Host:     "host.local",
	})
	if err == nil {
		t.Fatal("NormalizeCandidate() error = nil, want missing port error")
	}
}
