package engine

import (
	"context"
)

// Engine is the media-engine boundary. The service depends on this
// interface — never on HTTP details — so tests inject fakes and the
// transport (HTTP today, gRPC later) stays replaceable.
//
// Assumed engine API (implemented by the future external engine, NOT
// here): POST /v1/jobs, GET /v1/jobs/:jobID, POST /v1/jobs/:jobID/cancel.
type Engine interface {
	CreateJob(ctx context.Context, request CreateJobRequest) (CreateJobResponse, error)
	GetJob(ctx context.Context, engineJobID string) (EngineJobStatus, error)
	CancelJob(ctx context.Context, engineJobID string) error
}
