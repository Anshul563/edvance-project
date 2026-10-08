package storage

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	"github.com/Anshul563/edvance-project/services/video-service/internal/config"
)

// S3Storage talks to any S3-compatible endpoint. The same code path
// serves R2, S3 and MinIO: only the endpoint/region/credentials and
// the optional public base URL change through configuration.
type S3Storage struct {
	client    *s3.Client
	presigner *s3.PresignClient
	bucket    string
	publicURL string
	urlTTL    time.Duration
}

// NewS3 builds an ObjectStorage from configuration. Static
// credentials are used explicitly so no ambient credential chain
// (instance profiles, env defaults) can silently take over.
func NewS3(cfg config.StorageConfig, urlTTL time.Duration) (*S3Storage, error) {
	awsCfg := aws.Config{
		Region: cfg.Region,
		Credentials: credentials.NewStaticCredentialsProvider(
			cfg.AccessKeyID,
			cfg.SecretAccessKey,
			"",
		),
	}

	if cfg.Endpoint != "" {
		awsCfg.BaseEndpoint = aws.String(cfg.Endpoint)
	}

	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		// Custom endpoints (MinIO, R2) are reached path-style;
		// virtual-host style needs real DNS for the bucket.
		if cfg.Endpoint != "" {
			options.UsePathStyle = true
		}
	})

	return &S3Storage{
		client:    client,
		presigner: s3.NewPresignClient(client),
		bucket:    cfg.Bucket,
		publicURL: strings.TrimRight(cfg.PublicURL, "/"),
		urlTTL:    urlTTL,
	}, nil
}

// CreateUploadURL presigns a PUT whose signature binds the content
// type, so the validated MIME type is also enforced at the store.
func (s *S3Storage) CreateUploadURL(
	ctx context.Context,
	key string,
	contentType string,
) (string, error) {
	result, err := s.presigner.PresignPutObject(
		ctx,
		&s3.PutObjectInput{
			Bucket:      aws.String(s.bucket),
			Key:         aws.String(key),
			ContentType: aws.String(contentType),
		},
		s3.WithPresignExpires(s.urlTTL),
	)
	if err != nil {
		return "", fmt.Errorf("presign upload: %w", err)
	}

	return result.URL, nil
}

// Exists probes the object with HEAD. A missing object is a normal
// answer (false), not an error.
func (s *S3Storage) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err == nil {
		return true, nil
	}

	var notFound smithy.APIError

	if errors.As(err, &notFound) {
		switch notFound.ErrorCode() {
		case "NotFound", "NoSuchKey", "404":
			return false, nil
		}
	}

	// Some gateways answer HEAD with a bare 404 status.
	var statusErr *smithyhttp.ResponseError

	if errors.As(err, &statusErr) && statusErr.HTTPStatusCode() == http.StatusNotFound {
		return false, nil
	}

	return false, fmt.Errorf("head object: %w", err)
}

// Delete removes the object. S3 reports success for missing keys.
func (s *S3Storage) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("delete object: %w", err)
	}

	return nil
}

// GetURL prefers a stable public URL; when the store is private it
// falls back to a presigned GET.
func (s *S3Storage) GetURL(ctx context.Context, key string) (string, error) {
	if s.publicURL != "" {
		return s.publicURL + "/" + key, nil
	}

	result, err := s.presigner.PresignGetObject(
		ctx,
		&s3.GetObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(key),
		},
		s3.WithPresignExpires(s.urlTTL),
	)
	if err != nil {
		return "", fmt.Errorf("presign get: %w", err)
	}

	return result.URL, nil
}

// PublicURL exposes the configured public base URL (empty when the
// bucket is private), so callers can persist stable URLs instead of
// expiring signatures.
func (s *S3Storage) PublicURL() string {
	return s.publicURL
}
