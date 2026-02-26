package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// LocalStorage stores files on disk and serves them via a URL prefix.
type LocalStorage struct {
	dir    string // absolute path to upload directory
	prefix string // URL prefix, e.g. "/uploads"
}

// NewLocal creates a LocalStorage writing to dir, serving at prefix.
func NewLocal(dir, prefix string) *LocalStorage {
	os.MkdirAll(dir, 0755)
	return &LocalStorage{dir: dir, prefix: prefix}
}

// Upload saves r to disk with a unique name derived from filename.
func (s *LocalStorage) Upload(ctx context.Context, filename string, r io.Reader) (string, error) {
	ext := filepath.Ext(filename)
	unique := fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), sanitize(strings.TrimSuffix(filename, ext)), ext)
	dest := filepath.Join(s.dir, unique)

	f, err := os.Create(dest)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, r); err != nil {
		os.Remove(dest)
		return "", fmt.Errorf("write file: %w", err)
	}
	return s.prefix + "/" + unique, nil
}

// Delete removes the file corresponding to url from disk.
func (s *LocalStorage) Delete(ctx context.Context, url string) error {
	if !strings.HasPrefix(url, s.prefix+"/") {
		return nil // not a local file, ignore
	}
	filename := strings.TrimPrefix(url, s.prefix+"/")
	filename = filepath.Base(filename) // prevent path traversal
	return os.Remove(filepath.Join(s.dir, filename))
}

// sanitize removes unsafe characters from filenames.
func sanitize(name string) string {
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}
