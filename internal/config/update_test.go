package config

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
)

func TestUpdateSerializesConcurrentMutations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := Save(path, NewFile()); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	const workers = 12
	var wait sync.WaitGroup
	errorsCh := make(chan error, workers)
	for index := 0; index < workers; index++ {
		index := index
		wait.Add(1)
		go func() {
			defer wait.Done()
			errorsCh <- Update(context.Background(), path, func(file *File) error {
				file.Connections = append(file.Connections, Entry{
					Name:     "entry-" + string(rune('a'+index)),
					Protocol: "ssh",
					Host:     "example.com",
				})
				return nil
			})
		}()
	}
	wait.Wait()
	close(errorsCh)
	for err := range errorsCh {
		if err != nil {
			t.Fatalf("Update() error = %v", err)
		}
	}
	file, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(file.Connections) != workers {
		t.Fatalf("connections = %d, want %d", len(file.Connections), workers)
	}
}

func TestUpdateHonorsCanceledContextBeforeFilesystemChanges(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.yaml")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Update(ctx, path, func(*File) error { return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("Update() error = %v, want context.Canceled", err)
	}
}
