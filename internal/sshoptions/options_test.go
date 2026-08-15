package sshoptions

import (
	"strings"
	"testing"

	"github.com/Alurith/hoplane/internal/domain"
)

func TestDecodeRejectsSourceMetadataFromOptions(t *testing.T) {
	_, err := Decode(domain.Options{Namespace: {ConfigFile: "/tmp/config"}})
	if err == nil || !strings.Contains(err.Error(), "source metadata") {
		t.Fatalf("Decode() error = %v, want source metadata rejection", err)
	}
}

func TestDecodeMetadataRequiresCompleteReference(t *testing.T) {
	_, err := DecodeMetadata(domain.Metadata{Namespace: {ConfigFile: "/tmp/config"}})
	if err == nil {
		t.Fatal("DecodeMetadata() error = nil, want incomplete metadata rejection")
	}
}
