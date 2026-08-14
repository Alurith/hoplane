package catalog

import (
	"context"
	"testing"

	"github.com/Alurith/hoplane/internal/config"
	"github.com/Alurith/hoplane/internal/discovery"
)

func TestBuildNormalizesAndSortsStaticConnections(t *testing.T) {
	file := config.NewFile()
	file.Connections = []config.Entry{
		{Name: "zulu", Protocol: "vnc", Host: "display.local"},
		{Name: "alpha", Protocol: "ssh", Host: "nas.local"},
	}

	catalog, err := Build(context.Background(), discovery.NewStaticSource(file, "config.yaml"))
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(catalog.Connections) != 2 {
		t.Fatalf("connections = %d, want 2", len(catalog.Connections))
	}
	if catalog.Connections[0].Name != "alpha" || catalog.Connections[1].Name != "zulu" {
		t.Fatalf("names = %q, %q", catalog.Connections[0].Name, catalog.Connections[1].Name)
	}
	if catalog.Connections[1].Endpoint.Port != 5900 {
		t.Fatalf("VNC port = %d, want 5900", catalog.Connections[1].Endpoint.Port)
	}
}
