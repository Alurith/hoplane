package rdpoptions

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Alurith/hoplane/internal/domain"
)

func TestDecodeRDPNamespace(t *testing.T) {
	got, err := Decode(domain.Options{
		Namespace: {
			Domain:            "CONTOSO",
			Fullscreen:        "true",
			IgnoreCertificate: "false",
		},
	})
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	want := Options{Domain: "CONTOSO", Fullscreen: true}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Decode() = %#v, want %#v", got, want)
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	want := Options{Domain: "CONTOSO", Fullscreen: true, IgnoreCertificate: true}
	got, err := Decode(Encode(want))
	if err != nil {
		t.Fatalf("Decode(Encode()) error = %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip = %#v, want %#v", got, want)
	}
}

func TestDecodeRejectsEmptyDomain(t *testing.T) {
	_, err := Decode(domain.Options{Namespace: {Domain: "  "}})
	if err == nil || !strings.Contains(err.Error(), `RDP option "domain" cannot be empty`) {
		t.Fatalf("Decode() error = %v, want empty domain error", err)
	}
}

func TestDecodeRejectsInvalidBoolean(t *testing.T) {
	_, err := Decode(domain.Options{Namespace: {Fullscreen: "yes"}})
	if err == nil || !strings.Contains(err.Error(), `RDP option "fullscreen" must be a boolean`) {
		t.Fatalf("Decode() error = %v, want invalid boolean error", err)
	}
}

func TestDecodeRejectsUnknownOption(t *testing.T) {
	_, err := Decode(domain.Options{Namespace: {"unknown": "value"}})
	if err == nil || !strings.Contains(err.Error(), `unsupported RDP option "unknown"`) {
		t.Fatalf("Decode() error = %v, want unknown option error", err)
	}
}

func TestDecodeRejectsEmptyValue(t *testing.T) {
	_, err := Decode(domain.Options{Namespace: {Domain: "  "}})
	if err == nil || !strings.Contains(err.Error(), `RDP option "domain" cannot be empty`) {
		t.Fatalf("Decode() error = %v, want empty value error", err)
	}
}

func TestDecodeRejectsOtherNamespaces(t *testing.T) {
	_, err := Decode(domain.Options{
		"ssh":     {"identity_file": "/tmp/id"},
		Namespace: {Fullscreen: "true"},
	})
	if err == nil || !strings.Contains(err.Error(), `options namespace "ssh" is not valid for RDP`) {
		t.Fatalf("Decode() error = %v, want incompatible namespace error", err)
	}
}
