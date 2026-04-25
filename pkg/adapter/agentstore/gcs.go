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

var _ Writer = (*gcsWriter)(nil)

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

// Put returns the GCS object writer. Calling Close commits the upload;
// calling Abort cancels the per-Put context so the in-flight upload is torn
// down without finalising a truncated object.
func (b *GCSBackend) Put(ctx context.Context, key string) (Writer, error) {
	putCtx, cancel := context.WithCancel(ctx)
	w := b.client.Bucket(b.bucket).Object(b.objectName(key)).NewWriter(putCtx)
	w.ContentType = "application/json"
	return &gcsWriter{w: w, cancel: cancel}, nil
}

type gcsWriter struct {
	w      *storage.Writer
	cancel context.CancelFunc
	done   bool
}

func (g *gcsWriter) Write(p []byte) (int, error) { return g.w.Write(p) }

func (g *gcsWriter) Close() error {
	if g.done {
		return nil
	}
	g.done = true
	err := g.w.Close()
	g.cancel()
	return err
}

func (g *gcsWriter) Abort(_ error) {
	if g.done {
		return
	}
	g.done = true
	// Cancel the per-Put context first so the SDK aborts the resumable
	// upload, then drain the writer's Close to release any internal state.
	g.cancel()
	_ = g.w.Close()
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
