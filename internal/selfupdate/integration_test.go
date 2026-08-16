package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	goselfupdate "github.com/creativeprojects/go-selfupdate"
)

func TestPublishedArchiveContract(t *testing.T) {
	const (
		assetName = "hoplane_1.1.0_linux_amd64.tar.gz"
		binary    = "new hoplane binary"
	)

	archive := tarGzip(t, "hoplane", []byte(binary))
	hash := sha256.Sum256(archive)
	checksum := fmt.Sprintf("%x  %s\n", hash, assetName)
	manifest := fmt.Sprintf(`releases:
  - id: 1
    tag_name: v1.1.0
    assets:
      - id: 1
        name: %s
        size: %d
        url: v1.1.0/%s
      - id: 2
        name: checksums.txt
        size: %d
        url: v1.1.0/checksums.txt
`, assetName, len(archive), assetName, len(checksum))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/Alurith/hoplane/manifest.yaml":
			_, _ = w.Write([]byte(manifest))
		case "/Alurith/hoplane/v1.1.0/" + assetName:
			_, _ = w.Write(archive)
		case "/Alurith/hoplane/v1.1.0/checksums.txt":
			_, _ = w.Write([]byte(checksum))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	source, err := goselfupdate.NewHttpSource(goselfupdate.HttpConfig{BaseURL: server.URL + "/"})
	if err != nil {
		t.Fatalf("NewHttpSource() error = %v", err)
	}
	updater, err := goselfupdate.NewUpdater(goselfupdate.Config{
		Source: source,
		OS:     "linux",
		Arch:   "amd64",
		Validator: &goselfupdate.ChecksumValidator{
			UniqueFilename: "checksums.txt",
		},
	})
	if err != nil {
		t.Fatalf("NewUpdater() error = %v", err)
	}

	release, found, err := updater.DetectLatest(context.Background(), goselfupdate.NewRepositorySlug("Alurith", "hoplane"))
	if err != nil {
		t.Fatalf("DetectLatest() error = %v", err)
	}
	if !found {
		t.Fatal("DetectLatest() found = false, want true")
	}

	target := filepath.Join(t.TempDir(), "hoplane")
	if err := os.WriteFile(target, []byte("old hoplane binary"), 0o755); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := updater.UpdateTo(context.Background(), release, target); err != nil {
		t.Fatalf("UpdateTo() error = %v", err)
	}
	updated, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(updated) != binary {
		t.Fatalf("updated binary = %q, want %q", updated, binary)
	}
}

func tarGzip(t *testing.T, name string, contents []byte) []byte {
	t.Helper()
	var archive bytes.Buffer
	gzipWriter := gzip.NewWriter(&archive)
	tarWriter := tar.NewWriter(gzipWriter)
	if err := tarWriter.WriteHeader(&tar.Header{
		Name: name,
		Mode: 0o755,
		Size: int64(len(contents)),
	}); err != nil {
		t.Fatalf("WriteHeader() error = %v", err)
	}
	if _, err := tarWriter.Write(contents); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := tarWriter.Close(); err != nil {
		t.Fatalf("tar Close() error = %v", err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatalf("gzip Close() error = %v", err)
	}
	return archive.Bytes()
}
