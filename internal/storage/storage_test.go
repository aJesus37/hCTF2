package storage_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yourusername/hctf2/internal/storage"
)

func TestLocalStorage_UploadAndDelete(t *testing.T) {
	dir := t.TempDir()
	s := storage.NewLocal(dir, "/uploads")

	content := []byte("hello world")
	url, err := s.Upload(context.Background(), "test.txt", bytes.NewReader(content))
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}
	if !strings.HasPrefix(url, "/uploads/") {
		t.Fatalf("expected URL to start with /uploads/, got %s", url)
	}

	// Verify file exists
	filename := filepath.Base(url)
	if _, err := os.Stat(filepath.Join(dir, filename)); err != nil {
		t.Fatalf("file not found on disk: %v", err)
	}

	// Delete
	if err := s.Delete(context.Background(), url); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, filename)); !os.IsNotExist(err) {
		t.Fatal("expected file to be deleted")
	}
}
