package selfupdate

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

type fakeBackend struct {
	candidate      releaseCandidate
	found          bool
	detectErr      error
	updateErr      error
	detectCalls    int
	updateCalls    int
	updatePath     string
	updatedVersion string
	candidates     []releaseCandidate
}

func (f *fakeBackend) detectLatest(context.Context) (releaseCandidate, bool, error) {
	f.detectCalls++
	if len(f.candidates) > 0 {
		candidate := f.candidates[0]
		f.candidates = f.candidates[1:]
		return candidate, true, f.detectErr
	}
	return f.candidate, f.found, f.detectErr
}

func (f *fakeBackend) update(_ context.Context, candidate releaseCandidate, path string) error {
	f.updateCalls++
	f.updatePath = path
	f.updatedVersion = candidate.version
	return f.updateErr
}

func TestUpdateWithRejectsDevelopmentBuild(t *testing.T) {
	backend := &fakeBackend{}
	_, err := updateWith(context.Background(), DevelopmentVersion, backend, func() (string, error) {
		t.Fatal("executable path should not be resolved")
		return "", nil
	})
	if !errors.Is(err, ErrDevelopmentBuild) {
		t.Fatalf("error = %v, want ErrDevelopmentBuild", err)
	}
	if backend.detectCalls != 0 {
		t.Fatalf("detect calls = %d, want 0", backend.detectCalls)
	}
}

func TestUpdateWithRejectsInvalidVersion(t *testing.T) {
	backend := &fakeBackend{}
	_, err := updateWith(context.Background(), "not-a-version", backend, nil)
	if err == nil {
		t.Fatal("error = nil, want invalid version error")
	}
	if backend.detectCalls != 0 {
		t.Fatalf("detect calls = %d, want 0", backend.detectCalls)
	}
}

func TestUpdateWithReturnsNotFound(t *testing.T) {
	backend := &fakeBackend{}
	_, err := updateWith(context.Background(), "1.0.0", backend, nil)
	if !errors.Is(err, ErrReleaseNotFound) {
		t.Fatalf("error = %v, want ErrReleaseNotFound", err)
	}
}

func TestUpdateWithDoesNotUpdateWhenCurrentIsAtLeastLatest(t *testing.T) {
	for _, test := range []struct {
		name    string
		current string
		latest  string
	}{
		{name: "equal", current: "1.0.0", latest: "1.0.0"},
		{name: "newer", current: "2.0.0", latest: "1.0.0"},
	} {
		t.Run(test.name, func(t *testing.T) {
			backend := &fakeBackend{candidate: releaseCandidate{version: test.latest}, found: true}
			result, err := updateWith(context.Background(), test.current, backend, func() (string, error) {
				t.Fatal("executable path should not be resolved")
				return "", nil
			})
			if err != nil {
				t.Fatalf("error = %v", err)
			}
			if result.Updated {
				t.Fatal("Updated = true, want false")
			}
			if backend.updateCalls != 0 {
				t.Fatalf("update calls = %d, want 0", backend.updateCalls)
			}
		})
	}
}

func TestUpdateWithInstallsNewerRelease(t *testing.T) {
	backend := &fakeBackend{candidate: releaseCandidate{version: "v1.1.0"}, found: true}
	path := filepath.Join(t.TempDir(), "hoplane")
	result, err := updateWith(context.Background(), "1.0.0", backend, func() (string, error) {
		return path, nil
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if !result.Updated || result.CurrentVersion != "1.0.0" || result.LatestVersion != "1.1.0" {
		t.Fatalf("result = %#v", result)
	}
	if backend.updateCalls != 1 || backend.updatePath != path {
		t.Fatalf("update calls/path = %d/%q", backend.updateCalls, backend.updatePath)
	}
}

func TestUpdateWithUsesReleaseDetectedAfterLock(t *testing.T) {
	backend := &fakeBackend{
		candidates: []releaseCandidate{
			{version: "1.1.0"},
			{version: "1.2.0"},
		},
	}
	path := filepath.Join(t.TempDir(), "hoplane")
	result, err := updateWith(context.Background(), "1.0.0", backend, func() (string, error) {
		return path, nil
	})
	if err != nil {
		t.Fatalf("error = %v", err)
	}
	if !result.Updated || result.LatestVersion != "1.2.0" || backend.updatedVersion != "1.2.0" {
		t.Fatalf("result/version = %#v/%q", result, backend.updatedVersion)
	}
	if backend.detectCalls != 2 {
		t.Fatalf("detect calls = %d, want 2", backend.detectCalls)
	}
}

func TestUpdateWithPropagatesInstallError(t *testing.T) {
	installErr := errors.New("checksum failed")
	backend := &fakeBackend{
		candidate: releaseCandidate{version: "1.1.0"},
		found:     true,
		updateErr: installErr,
	}
	path := filepath.Join(t.TempDir(), "hoplane")
	_, err := updateWith(context.Background(), "1.0.0", backend, func() (string, error) {
		return path, nil
	})
	if !errors.Is(err, installErr) {
		t.Fatalf("error = %v, want install error", err)
	}
}

func TestUpdateWithPropagatesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	backend := &fakeBackend{}
	_, err := updateWith(ctx, "1.0.0", backend, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if backend.detectCalls != 0 {
		t.Fatalf("detect calls = %d, want 0", backend.detectCalls)
	}
}
