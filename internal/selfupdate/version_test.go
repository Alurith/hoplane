package selfupdate

import (
	"context"
	"path/filepath"
	"testing"
)

func TestUpdateRejectsInvalidReleaseVersion(t *testing.T) {
	backend := &fakeBackend{candidate: releaseCandidate{version: "not-a-version"}, found: true}
	_, err := updateWith(context.Background(), "1.0.0", backend, func() (string, error) {
		t.Fatal("invalid release must not resolve executable")
		return "", nil
	})
	if err == nil || backend.updateCalls != 0 {
		t.Fatalf("invalid release result: err=%v, updates=%d", err, backend.updateCalls)
	}
}

func TestUpdateDoesNotInstallReleaseThatIsNoLongerNewerAfterLock(t *testing.T) {
	backend := &fakeBackend{candidates: []releaseCandidate{{version: "1.1.0"}, {version: "1.0.0"}}}
	path := filepath.Join(t.TempDir(), "hoplane")
	result, err := updateWith(context.Background(), "1.0.0", backend, func() (string, error) { return path, nil })
	if err != nil || result.Updated || result.LatestVersion != "1.0.0" || backend.updateCalls != 0 {
		t.Fatalf("post-lock release result: result=%#v, err=%v, updates=%d", result, err, backend.updateCalls)
	}
}
