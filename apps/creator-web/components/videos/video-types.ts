export type VideoStatus =
  | "draft"
  | "upload_pending"
  | "uploaded"
  | "processing"
  | "ready"
  | "published"
  | "failed"
  | "archived"

export type VideoVisibility = "private" | "unlisted" | "public"

export type ModerationStatus = "approved" | "pending_review" | "rejected" | "held"

export interface VideoTag {
  id: string
  name: string
  slug: string
}

export interface Video {
  id: string
  creatorId: string
  title: string
  description?: string
  slug: string
  language: string
  categoryId?: string
  visibility: VideoVisibility
  status: VideoStatus
  moderationStatus: ModerationStatus
  moderationReason?: string
  mediaAssetId?: string
  thumbnailUrl?: string
  durationSeconds?: number
  processingErrorCode?: string
  processingErrorMessage?: string
  viewCount: number
  likeCount: number
  commentCount: number
  publishedAt?: string
  createdAt: string
  updatedAt: string
  tags: VideoTag[]
}

export interface VideoPage {
  items: Video[]
  page: number
  limit: number
  total: number
  hasNext: boolean
}

export interface VideoCategory {
  id: string
  name: string
  slug: string
  description?: string
  parentId?: string
}

export interface CreatorProfile {
  id: string
  userId: string
  status: string
  displayName: string
}

export interface PlaybackCaption {
  language: string
  label: string
  format: string
  url?: string
  isDefault: boolean
}

export interface VideoPlayback {
  mediaAssetId: string
  status: string
  durationSeconds?: number
  width?: number
  height?: number
  playbackUrl?: string
  thumbnailUrl?: string
  captions?: PlaybackCaption[]
}

export interface MediaAsset {
  id: string
  type: "video" | "audio" | "image"
  originalFilename: string
  mimeType: string
  fileSizeBytes: number
  status: string
  durationSeconds?: number
}

export interface UploadSession {
  uploadUrl: string
  contentType: string
  asset: MediaAsset
}

export interface CreateVideoInput {
  title: string
  description?: string
  visibility?: VideoVisibility
  language?: string
  categorySlug?: string
  mediaAssetId?: string
  tags?: string[]
}

export type UpdateVideoInput = Partial<Omit<CreateVideoInput, "tags">> & {
  tags?: string[]
  thumbnailUrl?: string
}

export interface ListVideosParams {
  page?: number
  limit?: number
  creatorId?: string
  status?: VideoStatus
  visibility?: VideoVisibility
  category?: string
  language?: string
  tag?: string
  q?: string
  sort?: "newest" | "oldest" | "popular"
  signal?: AbortSignal
}

export const VIDEO_STATUS_LABELS: Record<VideoStatus, string> = {
  draft: "Draft",
  upload_pending: "Waiting for upload",
  uploaded: "Uploaded",
  processing: "Processing",
  ready: "Ready to publish",
  published: "Published",
  failed: "Processing failed",
  archived: "Archived",
}

export const VIDEO_VISIBILITY_LABELS: Record<VideoVisibility, string> = {
  private: "Private",
  unlisted: "Unlisted",
  public: "Public",
}

export const MODERATION_LABELS: Record<ModerationStatus, string> = {
  approved: "Approved",
  pending_review: "In review",
  rejected: "Rejected",
  held: "On hold",
}

export const VIDEO_LANGUAGES = [
  { value: "en", label: "English" },
  { value: "en-US", label: "English (US)" },
  { value: "en-GB", label: "English (UK)" },
  { value: "es", label: "Spanish" },
  { value: "fr", label: "French" },
  { value: "de", label: "German" },
  { value: "pt", label: "Portuguese" },
  { value: "hi", label: "Hindi" },
  { value: "ar", label: "Arabic" },
  { value: "ja", label: "Japanese" },
  { value: "zh", label: "Chinese" },
] as const

/** Matches the video-service presign contract (video/* only in the studio). */
export const ACCEPTED_VIDEO_TYPES = [
  "video/mp4",
  "video/webm",
  "video/quicktime",
  "video/x-matroska",
] as const

export const ACCEPTED_VIDEO_EXTENSIONS = ".mp4,.webm,.mov,.mkv"

/** The video-service default (MAX_VIDEO_SIZE_BYTES); server re-validates. */
export const MAX_VIDEO_SIZE_BYTES = 2 * 1024 * 1024 * 1024

export function formatBytes(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B"
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`
}

export function formatDuration(seconds?: number): string {
  if (!seconds || seconds <= 0) return "—"
  const total = Math.floor(seconds)
  const hours = Math.floor(total / 3600)
  const minutes = Math.floor((total % 3600) / 60)
  const secs = total % 60
  if (hours > 0) {
    return `${hours}:${String(minutes).padStart(2, "0")}:${String(secs).padStart(2, "0")}`
  }
  return `${minutes}:${String(secs).padStart(2, "0")}`
}

export function formatDateTime(value?: string): string {
  if (!value) return "—"
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return "—"
  return date.toLocaleString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  })
}

export function isProcessing(status: VideoStatus): boolean {
  return (
    status === "upload_pending" ||
    status === "uploaded" ||
    status === "processing"
  )
}

export function canPublish(video: Video): boolean {
  return (
    video.status === "ready" &&
    video.moderationStatus === "approved" &&
    Boolean(video.mediaAssetId)
  )
}

export function canRetryProcessing(video: Video): boolean {
  return video.status === "failed" && Boolean(video.mediaAssetId)
}
