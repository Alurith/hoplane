package catalog

import (
	"context"
	"errors"
	"testing"

	"github.com/Alurith/hoplane/internal/config"
)

func TestBuildNormalizesAndSortsStaticConnections(t *testing.T) {
	file := config.NewFile()
	file.Connections = []config.Entry{
		{Name: "zulu", Protocol: "rdp", Host: "display.local"},
		{Name: "alpha", Protocol: "ssh", Host: "nas.local"},
	}

	connections, err := Build(context.Background(), file, "config.yaml")
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(connections) != 2 {
		t.Fatalf("connections = %d, want 2", len(connections))
	}
	if connections[0].Name != "alpha" || connections[1].Name != "zulu" {
		t.Fatalf("names = %q, %q", connections[0].Name, connections[1].Name)
	}
	if connections[1].Endpoint.Port != 3389 {
		t.Fatalf("RDP port = %d, want 3389", connections[1].Endpoint.Port)
	}
	if connections[0].Endpoint.Source.Name != "static" || connections[0].Endpoint.Source.ID != "config.yaml" {
		t.Fatalf("source = %#v, want static config provenance", connections[0].Endpoint.Source)
	}
}

func TestBuildHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Build(ctx, config.NewFile(), "config.yaml"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Build() error = %v, want context.Canceled", err)
	}
}
