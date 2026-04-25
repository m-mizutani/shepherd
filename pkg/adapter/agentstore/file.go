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

func (b *FileBackend) resolve(key string) (string, error) {
	if key == "" {
		return "", goerr.New("storage key must not be empty")
	}
	parts := strings.Split(key, "/")
	for _, p := range parts {
		// Reject anything that could escape the base directory. Callers
		// already use safe segments (UUIDs, workspace/ticket IDs) so this is
		// a defence-in-depth check that also satisfies gosec G304.
		if p == "" || p == "." || p == ".." || strings.ContainsRune(p, os.PathSeparator) {
			return "", goerr.New("invalid path segment in storage key", goerr.V("key", key))
		}
	}
	all := append([]string{b.base}, parts...)
	return filepath.Join(all...), nil
}

// Put returns a writer that creates (or replaces) the file at the resolved
// path. The caller must Close the writer to commit, or Abort to delete the
// half-written file on mid-stream failure.
//
// The file is created with 0o600 permissions: agent history may contain
// internal ticket discussion that should not be world-readable.
func (b *FileBackend) Put(_ context.Context, key string) (Writer, error) {
	path, err := b.resolve(key)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, goerr.Wrap(err, "failed to create directory",
			goerr.V("path", filepath.Dir(path)))
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0o600) // #nosec G304 -- path is validated by resolve().
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create file", goerr.V("path", path))
	}
	return &fileWriter{f: f, path: path}, nil
}

// fileWriter wraps *os.File so that Abort closes the descriptor and removes
// the partially written file. Close commits the file as written.
type fileWriter struct {
	f      *os.File
	path   string
	done   bool
}

func (w *fileWriter) Write(p []byte) (int, error) { return w.f.Write(p) }

func (w *fileWriter) Close() error {
	if w.done {
		return nil
	}
	w.done = true
	return w.f.Close()
}

func (w *fileWriter) Abort(_ error) {
	if w.done {
		return
	}
	w.done = true
	_ = w.f.Close()
	_ = os.Remove(w.path)
}

// Get opens the file at the resolved path. Returns (nil, nil) if the file
// does not exist.
func (b *FileBackend) Get(_ context.Context, key string) (io.ReadCloser, error) {
	path, err := b.resolve(key)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path) // #nosec G304 -- path is validated by resolve().
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
