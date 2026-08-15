package tui

import (
	"strings"
	"testing"

	"github.com/Alurith/hoplane/internal/domain"
)

func TestRDPWarningAppearsBeforeDescription(t *testing.T) {
	item := NewItem(domain.Connection{
		Endpoint:    domain.Endpoint{Protocol: domain.ProtocolRDP, Host: "desktop.example.com", Port: 3389},
		Description: "description",
		Options:     domain.Options{"rdp": {"ignore_certificate": "true"}},
	})
	lines := strings.Split(item.Description(), "\n")
	if len(lines) < 3 || !strings.Contains(lines[1], "certificate validation disabled") {
		t.Fatalf("description = %q, want visible warning before description", item.Description())
	}
}
