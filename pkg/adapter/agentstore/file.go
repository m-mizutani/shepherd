package agentstore

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/m-mizutani/goerr/v2"
)

// FileBackend persists agent data on the local filesystem under a base
// directory. Keys are interpreted as POSIX-style relative paths (forward
// slashes) and translated to OS-native paths.
type FileBackend struct {
	base string
}

// NewFileBackend returns a FileBackend rooted at dir. The directory is created
// lazily on first write.
func NewFileBackend(dir string) (*FileBackend, error) {
	if dir == "" {
		return nil, goerr.New("file backend directory must not be empty")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to resolve absolute path", goerr.V("dir", dir))
	}
	return &FileBackend{base: abs}, nil
}

func (b *FileBackend) resolve(key string) string {
	parts := strings.Split(key, "/")
	all := append([]string{b.base}, parts...)
	return filepath.Join(all...)
}

// Put returns a writer that creates (or replaces) the file at the resolved
// path. The caller must Close the writer to flush the data.
func (b *FileBackend) Put(_ context.Context, key string) (io.WriteCloser, error) {
	path := b.resolve(key)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, goerr.Wrap(err, "failed to create directory",
			goerr.V("path", filepath.Dir(path)))
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create file", goerr.V("path", path))
	}
	return f, nil
}

// Get opens the file at the resolved path. Returns (nil, nil) if the file
// does not exist.
func (b *FileBackend) Get(_ context.Context, key string) (io.ReadCloser, error) {
	path := b.resolve(key)
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, goerr.Wrap(err, "failed to open file", goerr.V("path", path))
	}
	return f, nil
}

// Close is a no-op for the file backend.
func (b *FileBackend) Close() error { return nil }
