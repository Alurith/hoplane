//go:build linux

package connector

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Alurith/hoplane/internal/domain"
)

func TestLinuxRDPConnectorPlansXFREERDP3Invocations(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		user     string
		options  domain.Options
		wantArgs []string
	}{
		{
			name:     "DNS with user",
			host:     "desktop.example.com",
			user:     "alice",
			wantArgs: []string{"/v:desktop.example.com:3389", "/u:alice"},
		},
		{
			name: "IPv4 without user and explicit client",
			host: "192.0.2.10",
			options: domain.Options{"rdp": {
				"client": "xfreerdp3",
			}},
			wantArgs: []string{"/v:192.0.2.10:3389"},
		},
		{
			name: "IPv6 fullscreen with certificate validation disabled",
			host: "2001:db8::10",
			user: "alice",
			options: domain.Options{"rdp": {
				"fullscreen":         "true",
				"ignore_certificate": "true",
			}},
			wantArgs: []string{"/v:[2001:db8::10]:3389", "/u:alice", "/f", "/cert:ignore"},
		},
		{
			name: "local account domain",
			host: "192.0.2.10",
			user: "aluri",
			options: domain.Options{"rdp": {
				"domain": "LAPTOP-25TLFPF0",
			}},
			wantArgs: []string{"/v:192.0.2.10:3389", "/u:aluri", "/d:LAPTOP-25TLFPF0"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			connection := rdpConnection()
			connection.Endpoint.Host = test.host
			connection.Endpoint.User = test.user
			connection.Options = test.options

			got, err := NewRDPConnector(&fakeRunner{}).Plan(connection)
			if err != nil {
				t.Fatalf("Plan() error = %v", err)
			}
			want := Invocation{Program: "xfreerdp3", Args: test.wantArgs}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("Plan() = %#v, want %#v", got, want)
			}
		})
	}
}

func TestLinuxRDPConnectorLooksUpXFREERDP3(t *testing.T) {
	runner := &fakeRunner{path: "/usr/bin/xfreerdp3"}
	if err := NewRDPConnector(runner).Connect(context.Background(), rdpConnection(), IO{}); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	if runner.lookedUp != "xfreerdp3" {
		t.Fatalf("LookPath() name = %q, want xfreerdp3", runner.lookedUp)
	}
	if runner.invocation.Program != "/usr/bin/xfreerdp3" {
		t.Fatalf("program = %q, want /usr/bin/xfreerdp3", runner.invocation.Program)
	}
}

func TestLinuxRDPConnectorReportsMissingXFREERDP3(t *testing.T) {
	runner := &fakeRunner{lookupErr: errors.New("not found")}
	err := NewRDPConnector(runner).Connect(context.Background(), rdpConnection(), IO{})
	if !errors.Is(err, ErrClientUnavailable) || !strings.Contains(err.Error(), `executable "xfreerdp3" for RDP client "xfreerdp3" is not available`) {
		t.Fatalf("Connect() error = %v, want missing xfreerdp3 error", err)
	}
	if runner.runCalled {
		t.Fatal("Run() was called after missing xfreerdp3")
	}
}
