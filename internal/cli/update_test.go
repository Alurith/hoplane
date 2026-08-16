package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Alurith/hoplane/internal/selfupdate"
)

func TestUpdateCommandReportsUpdatedVersion(t *testing.T) {
	var output bytes.Buffer
	var gotVersion string
	err := Execute(context.Background(), []string{"update"}, Dependencies{
		Input:   bytes.NewBuffer(nil),
		Output:  &output,
		Errors:  &bytes.Buffer{},
		Version: "1.0.0",
		Update: func(_ context.Context, version string) (selfupdate.Result, error) {
			gotVersion = version
			return selfupdate.Result{CurrentVersion: "1.0.0", LatestVersion: "1.1.0", Updated: true}, nil
		},
	})
	if err != nil {
		t.Fatalf("update error = %v", err)
	}
	if gotVersion != "1.0.0" {
		t.Fatalf("version = %q, want 1.0.0", gotVersion)
	}
	if output.String() != "updated hoplane from 1.0.0 to 1.1.0\n" {
		t.Fatalf("output = %q", output.String())
	}
}

func TestUpdateCommandReportsCurrentVersion(t *testing.T) {
	var output bytes.Buffer
	err := Execute(context.Background(), []string{"update"}, Dependencies{
		Input:   bytes.NewBuffer(nil),
		Output:  &output,
		Errors:  &bytes.Buffer{},
		Version: "1.0.0",
		Update: func(context.Context, string) (selfupdate.Result, error) {
			return selfupdate.Result{CurrentVersion: "1.0.0", LatestVersion: "1.0.0"}, nil
		},
	})
	if err != nil {
		t.Fatalf("update error = %v", err)
	}
	if output.String() != "hoplane is up to date (current 1.0.0, latest 1.0.0)\n" {
		t.Fatalf("output = %q", output.String())
	}
}

func TestUpdateCommandPropagatesError(t *testing.T) {
	updateErr := errors.New("update failed")
	var output bytes.Buffer
	err := Execute(context.Background(), []string{"update"}, Dependencies{
		Input:   bytes.NewBuffer(nil),
		Output:  &output,
		Errors:  &bytes.Buffer{},
		Version: "1.0.0",
		Update: func(context.Context, string) (selfupdate.Result, error) {
			return selfupdate.Result{}, updateErr
		},
	})
	if !errors.Is(err, updateErr) {
		t.Fatalf("error = %v, want update error", err)
	}
	if output.Len() != 0 {
		t.Fatalf("output = %q, want empty", output.String())
	}
}

func TestVersionFlagUsesInjectedVersion(t *testing.T) {
	var output bytes.Buffer
	err := Execute(context.Background(), []string{"--version"}, Dependencies{
		Input:   bytes.NewBuffer(nil),
		Output:  &output,
		Errors:  &bytes.Buffer{},
		Version: "1.2.3",
	})
	if err != nil {
		t.Fatalf("version error = %v", err)
	}
	if !strings.Contains(output.String(), "hoplane 1.2.3") {
		t.Fatalf("output = %q", output.String())
	}
}
