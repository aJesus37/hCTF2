package storage

import (
	"context"
	"io"
)

// Storage handles file upload and deletion.
type Storage interface {
	Upload(ctx context.Context, filename string, r io.Reader) (url string, err error)
	Delete(ctx context.Context, url string) error
}
