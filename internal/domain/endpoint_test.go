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

func TestNormalizeCandidateClonesOptions(t *testing.T) {
	options := Options{"ssh": {"identity_file": "~/.ssh/id_ed25519"}}
	connection, err := NormalizeCandidate(Candidate{
		Name:     "nas",
		Protocol: "ssh",
		Host:     "nas.local",
		Options:  options,
	})
	if err != nil {
		t.Fatalf("NormalizeCandidate() error = %v", err)
	}

	options["ssh"]["identity_file"] = "changed"
	if got := connection.Options["ssh"]["identity_file"]; got != "~/.ssh/id_ed25519" {
		t.Fatalf("connection options were not cloned: %q", got)
	}
}

func TestNormalizeCandidateAcceptsBracketedIPv6(t *testing.T) {
	connection, err := NormalizeCandidate(Candidate{
		Name:     "router",
		Protocol: "ssh",
		Host:     "[2001:DB8::10]",
	})
	if err != nil {
		t.Fatalf("NormalizeCandidate() error = %v", err)
	}
	if connection.Endpoint.Host != "2001:db8::10" {
		t.Fatalf("host = %q, want normalized IPv6 address", connection.Endpoint.Host)
	}
}

func TestNormalizeCandidateRejectsInvalidBracketedOrTargetHosts(t *testing.T) {
	for _, host := range []string{"[]", "[host.local]", "-oProxyCommand=bad", "user@host.local"} {
		t.Run(host, func(t *testing.T) {
			_, err := NormalizeCandidate(Candidate{
				Name:     "invalid",
				Protocol: "ssh",
				Host:     host,
			})
			if err == nil {
				t.Fatalf("NormalizeCandidate() error = nil for host %q", host)
			}
		})
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
