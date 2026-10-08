// Package storage abstracts the object store behind upload, playback
// and cleanup operations. Implementations exist for any S3-compatible
// endpoint (Cloudflare R2, AWS S3, MinIO); no business logic knows
// which vendor is in use.
package storage

import "context"

// ObjectStorage is the seam between the video service and the byte
// store. CreateUploadURL returns a presigned PUT URL the client uses
// to upload directly — bytes never flow through this service.
type ObjectStorage interface {
	// CreateUploadURL returns a presigned PUT URL for key, valid for
	// the configured TTL. The content type is part of the signature,
	// so the uploader must send exactly the validated MIME type.
	CreateUploadURL(ctx context.Context, key string, contentType string) (string, error)

	// Exists reports whether the object is present (HEAD). Upload
	// completion must not trust the client's word that bytes arrived.
	Exists(ctx context.Context, key string) (bool, error)

	// Delete removes the object. Deleting a missing key is not an error.
	Delete(ctx context.Context, key string) error

	// GetURL returns a public URL when the store is publicly readable,
	// otherwise a presigned GET URL.
	GetURL(ctx context.Context, key string) (string, error)
}
