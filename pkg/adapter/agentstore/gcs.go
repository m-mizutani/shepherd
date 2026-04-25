package agentstore

import (
	"context"
	"errors"
	"io"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/m-mizutani/goerr/v2"
	"google.golang.org/api/option"
)

// GCSBackend persists agent data to a Google Cloud Storage bucket. All object
// names are joined with the configured prefix (which may be empty).
type GCSBackend struct {
	client *storage.Client
	bucket string
	prefix string
}

// NewGCSBackend returns a GCSBackend backed by a freshly constructed
// *storage.Client. The caller is responsible for invoking Close when the
// backend is no longer needed.
func NewGCSBackend(ctx context.Context, bucket, prefix string, opts ...option.ClientOption) (*GCSBackend, error) {
	if bucket == "" {
		return nil, goerr.New("gcs backend bucket must not be empty")
	}
	client, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create GCS client", goerr.V("bucket", bucket))
	}
	return &GCSBackend{
		client: client,
		bucket: bucket,
		prefix: normalizePrefix(prefix),
	}, nil
}

func normalizePrefix(p string) string {
	p = strings.TrimLeft(p, "/")
	if p == "" || strings.HasSuffix(p, "/") {
		return p
	}
	return p + "/"
}

func (b *GCSBackend) objectName(key string) string {
	return b.prefix + strings.TrimLeft(key, "/")
}

// Put returns the GCS object writer. Callers must Close the returned writer
// to finalize the upload; closing without writing yields an empty object, and
// abandoning the writer without Close discards the upload (the property we
// rely on for atomic JSON encoding — see history.go / trace.go).
func (b *GCSBackend) Put(ctx context.Context, key string) (io.WriteCloser, error) {
	w := b.client.Bucket(b.bucket).Object(b.objectName(key)).NewWriter(ctx)
	w.ContentType = "application/json"
	return w, nil
}

// Get opens the GCS object for reading. Returns (nil, nil) when the object
// does not exist.
func (b *GCSBackend) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	name := b.objectName(key)
	r, err := b.client.Bucket(b.bucket).Object(name).NewReader(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			return nil, nil
		}
		return nil, goerr.Wrap(err, "failed to read GCS object",
			goerr.V("bucket", b.bucket),
			goerr.V("object", name))
	}
	return r, nil
}

// Close releases the underlying GCS client.
func (b *GCSBackend) Close() error {
	return b.client.Close()
}
