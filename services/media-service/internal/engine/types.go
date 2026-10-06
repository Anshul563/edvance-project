package engine

// Types below mirror the assumed media-engine API contract. They live
// here — isolated from service/handler code — so contract drift is a
// one-file change.

// CreateJobRequest asks the engine to start processing.
type CreateJobRequest struct {
	Type    string
	Source  JobSource
	Options JobOptions
}

// JobSource locates the input media in object storage.
type JobSource struct {
	ObjectKey string
}

// JobOptions selects the outputs the engine must produce.
type JobOptions struct {
	GenerateThumbnail bool
	GenerateHLS       bool
}

// CreateJobResponse acknowledges acceptance with the engine's job id.
type CreateJobResponse struct {
	JobID string
}

// EngineJobStatus is the engine's view of a job.
type EngineJobStatus struct {
	JobID    string
	Status   string
	Progress int32
	Output   EngineJobOutput
	Error    EngineJobError
}

// EngineJobOutput carries successful results. Raw engine internals
// (stack traces, worker ids) must never be added here: only mapped,
// client-safe fields cross into media-service responses.
type EngineJobOutput struct {
	ManifestURL  string
	ThumbnailURL string
	Duration     int64
	Width        int32
	Height       int32
}

// EngineJobError carries a failure in normalized form.
type EngineJobError struct {
	Code    string
	Message string
}

// Known engine status names. Anything else is treated as unknown and
// never silently mapped into a local terminal state.
const (
	EngineQueued     = "queued"
	EngineProcessing = "processing"
	EngineRunning    = "running"
	EngineCompleted  = "completed"
	EngineFailed     = "failed"
	EngineCancelled  = "cancelled"
)
