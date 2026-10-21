"use client"

import { useCallback, useEffect, useRef, useState } from "react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import { toast } from "sonner"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  Alert02Icon,
  ArrowLeft01Icon,
  Delete01Icon,
  PencilEdit01Icon,
  PlayCircleIcon,
  Refresh01Icon,
} from "@hugeicons/core-free-icons"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { Separator } from "@/components/ui/separator"
import { ConfirmationDialog } from "@/components/confirmation-dialog"
import { cn } from "@/lib/utils"
import {
  MODERATION_LABELS,
  VIDEO_VISIBILITY_LABELS,
  canPublish,
  canRetryProcessing,
  formatDateTime,
  formatDuration,
  isProcessing,
  type Video,
  type VideoCategory,
  type VideoPlayback,
} from "./video-types"
import {
  deleteMediaAsset,
  deleteVideo,
  getPlayback,
  getVideo,
  listCategories,
  publishVideo,
  retryProcessing,
  unpublishVideo,
} from "./video-api"
import { ModerationBadge, VideoStatusBadge } from "./video-status"
import { VideoPlayer } from "./video-player"

const POLL_MS = 4000

type Action = "publish" | "unpublish" | "retry" | "delete" | "playback" | null

function Timeline({ video }: { video: Video }) {
  const order: { key: string; label: string; description: string }[] = [
    { key: "created", label: "Draft created", description: "Metadata saved." },
    { key: "upload", label: "Upload", description: "Secure upload to object storage." },
    { key: "processing", label: "Processing", description: "Transcoding, thumbnails, and streaming formats." },
    { key: "ready", label: "Ready", description: "Media processing finished successfully." },
    { key: "published", label: "Published", description: "Visible according to its visibility." },
  ]

  const rank: Record<string, number> = {
    draft: 0,
    upload_pending: 1,
    uploaded: 2,
    processing: 2,
    ready: 3,
    published: 4,
    failed: 2,
    archived: 0,
  }
  const current = rank[video.status] ?? 0
  const failed = video.status === "failed"

  return (
    <ol className="space-y-0" aria-label="Processing timeline">
      {order.map((item, index) => {
        const done = index < current || (index === current && video.status === "ready")
        const active = index === current && !done
        const isFailedStep = failed && index === 2
        return (
          <li key={item.key} className="flex gap-3">
            <div className="flex flex-col items-center">
              <span
                className={cn(
                  "mt-0.5 flex size-5 shrink-0 items-center justify-center border text-[10px] font-medium",
                  done && "border-emerald-300 bg-emerald-50 text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-400",
                  active && !failed && "border-primary bg-primary text-primary-foreground",
                  isFailedStep && "border-destructive bg-destructive/10 text-destructive",
                  !done && !active && !isFailedStep && "border-border text-muted-foreground"
                )}
              >
                {done ? "✓" : isFailedStep ? "!" : index + 1}
              </span>
              {index < order.length - 1 && (
                <span
                  className={cn(
                    "my-1 w-px flex-1",
                    done ? "bg-emerald-300 dark:bg-emerald-900" : "bg-border"
                  )}
                  aria-hidden="true"
                />
              )}
            </div>
            <div className="pb-4">
              <p className={cn("text-xs font-medium", !done && !active && "text-muted-foreground")}>
                {item.label}
              </p>
              <p className="mt-0.5 text-[11px] text-muted-foreground">{item.description}</p>
              {isFailedStep && video.processingErrorMessage && (
                <p className="mt-1 text-[11px] text-destructive">
                  {video.processingErrorMessage}
                  {video.processingErrorCode ? ` (${video.processingErrorCode})` : ""}
                </p>
              )}
            </div>
          </li>
        )
      })}
    </ol>
  )
}

function DetailRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-0.5 py-2 sm:flex-row sm:gap-4">
      <dt className="w-40 shrink-0 text-xs text-muted-foreground">{label}</dt>
      <dd className="min-w-0 text-xs">{value}</dd>
    </div>
  )
}

export function VideoDetail({ videoId }: { videoId: string }) {
  const router = useRouter()

  const [video, setVideo] = useState<Video | null>(null)
  const [categories, setCategories] = useState<VideoCategory[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [action, setAction] = useState<Action>(null)
  const [actionError, setActionError] = useState<string | null>(null)

  const [playback, setPlayback] = useState<VideoPlayback | null>(null)
  const [playbackError, setPlaybackError] = useState<string | null>(null)
  const [deleteOpen, setDeleteOpen] = useState(false)
  const [watchStatus, setWatchStatus] = useState(false)

  const videoRef = useRef<Video | null>(null)
  useEffect(() => {
    videoRef.current = video
  }, [video])

  const loadVideo = useCallback(
    async (options?: { silent?: boolean; signal?: AbortSignal }) => {
      const result = await getVideo(videoId, { signal: options?.signal })
      if (options?.signal?.aborted) return
      if (!result.success) {
        if (!options?.silent) {
          setError(result.error?.message ?? "This video could not be loaded.")
          setLoading(false)
        }
        return
      }
      setError(null)
      setLoading(false)
      setVideo(result.data)
      if (result.data.status !== "failed") setWatchStatus(false)
    },
    [videoId]
  )

  useEffect(() => {
    const controller = new AbortController()
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void loadVideo({ signal: controller.signal })
    return () => controller.abort()
  }, [loadVideo])

  useEffect(() => {
    let active = true
    void listCategories().then((result) => {
      if (!active || !result.success) return
      setCategories(result.data.items ?? [])
    })
    return () => {
      active = false
    }
  }, [])

  const status = video?.status
  useEffect(() => {
    if (!status) return
    if (!isProcessing(status) && !watchStatus) return

    const timer = setInterval(() => {
      void loadVideo({ silent: true })
    }, POLL_MS)

    return () => clearInterval(timer)
  }, [status, watchStatus, loadVideo])

  const loadPlayback = useCallback(
    async (options?: { signal?: AbortSignal }) => {
      if (!videoRef.current) return
      const result = await getPlayback(videoId, { signal: options?.signal })
      setAction((current) => (current === "playback" ? null : current))
      if (options?.signal?.aborted) return
      if (!result.success) {
        setPlaybackError(
          result.error?.status === 503
            ? "Playback is temporarily unavailable. Try again in a moment."
            : "A preview is not available yet."
        )
        return
      }
      if (!result.data.playbackUrl) {
        setPlaybackError("No playable rendition was returned for this video.")
        return
      }
      setPlayback(result.data)
    },
    [videoId]
  )

  const canPreview =
    status === "ready" || status === "published"

  function retryPreview() {
    setPlaybackError(null)
    setAction("playback")
    setPlayback(null)
    void loadPlayback()
  }

  useEffect(() => {
    if (!canPreview || playback) return
    const controller = new AbortController()
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void loadPlayback({ signal: controller.signal })
    return () => controller.abort()
    // Playback is refreshed explicitly after failures; only the first
    // successful transition into a playable state needs an automatic fetch.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [canPreview])

  async function handlePublish() {
    if (!video || action) return
    setAction("publish")
    setActionError(null)
    const result = await publishVideo(video.id)
    setAction(null)
    if (!result.success) {
      setActionError(result.error?.message ?? "The video could not be published.")
      return
    }
    setVideo(result.data)
    toast.success("Video published.")
  }

  async function handleUnpublish() {
    if (!video || action) return
    setAction("unpublish")
    setActionError(null)
    const result = await unpublishVideo(video.id)
    setAction(null)
    if (!result.success) {
      setActionError(result.error?.message ?? "The video could not be unpublished.")
      return
    }
    setVideo(result.data)
    toast.success("Video unpublished.")
  }

  async function handleRetry() {
    if (!video?.mediaAssetId || action) return
    setAction("retry")
    setActionError(null)
    const result = await retryProcessing(video.mediaAssetId)
    setAction(null)
    if (!result.success) {
      setActionError(result.error?.message ?? "Processing could not be restarted.")
      return
    }
    setWatchStatus(true)
    toast.success("Processing restarted.")
    await loadVideo({ silent: true })
  }

  async function handleDelete() {
    if (!video || action) return
    setAction("delete")
    setActionError(null)

    const result = await deleteVideo(video.id)
    if (!result.success) {
      setAction(null)
      setActionError(result.error?.message ?? "The video could not be deleted.")
      setDeleteOpen(false)
      return
    }

    if (video.mediaAssetId) {
      const media = await deleteMediaAsset(video.mediaAssetId)
      if (!media.success) {
        toast.warning("Video deleted, but media cleanup will finish in the background.")
      }
    }

    setAction(null)
    setDeleteOpen(false)
    toast.success("Video deleted.")
    router.push("/creator/videos")
  }

  if (loading) {
    return (
      <div className="mx-auto max-w-6xl space-y-4 p-4 sm:p-6">
        <Skeleton className="h-6 w-40" />
        <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_340px]">
          <Skeleton className="aspect-video w-full" />
          <div className="space-y-2">
            <Skeleton className="h-24 w-full" />
            <Skeleton className="h-40 w-full" />
          </div>
        </div>
      </div>
    )
  }

  if (error || !video) {
    return (
      <div className="mx-auto max-w-3xl p-4 sm:p-6">
        <div role="alert" className="border border-border bg-card p-6">
          <div className="flex items-start gap-3">
            <HugeiconsIcon
              icon={Alert02Icon}
              strokeWidth={1.8}
              className="mt-0.5 size-5 shrink-0 text-destructive"
            />
            <div>
              <h2 className="text-sm font-semibold">Could not load this video</h2>
              <p className="mt-1 text-xs text-muted-foreground">
                {error ?? "The video was not found."}
              </p>
              <div className="mt-4 flex gap-2">
                <Button
                  size="sm"
                  variant="outline"
                  onClick={() => {
                    setLoading(true)
                    void loadVideo()
                  }}
                >
                  Retry
                </Button>
                <Link href="/creator/videos">
                  <Button size="sm" variant="ghost">
                    Back to My Videos
                  </Button>
                </Link>
              </div>
            </div>
          </div>
        </div>
      </div>
    )
  }

  const categoryName = video.categoryId
    ? categories.find((category) => category.id === video.categoryId)?.name
    : undefined

  return (
    <div className="mx-auto max-w-6xl p-4 sm:p-6">
      <div className="mb-5 flex flex-wrap items-start justify-between gap-3">
        <div className="flex min-w-0 items-start gap-2">
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label="Back to videos"
            onClick={() => router.push("/creator/videos")}
          >
            <HugeiconsIcon icon={ArrowLeft01Icon} strokeWidth={1.8} />
          </Button>
          <div className="min-w-0">
            <h2 className="truncate text-lg font-semibold">{video.title}</h2>
            <div className="mt-1 flex flex-wrap items-center gap-1.5">
              <VideoStatusBadge status={video.status} />
              <ModerationBadge status={video.moderationStatus} />
              <span className="text-[11px] text-muted-foreground">
                {VIDEO_VISIBILITY_LABELS[video.visibility]}
              </span>
            </div>
          </div>
        </div>

        <div className="flex flex-wrap gap-1.5">
          <Link href={`/creator/videos/${video.id}/edit`}>
            <Button variant="outline" size="sm">
              <HugeiconsIcon icon={PencilEdit01Icon} strokeWidth={1.8} />
              Edit
            </Button>
          </Link>
          {canPublish(video) && (
            <Button size="sm" disabled={Boolean(action)} onClick={() => void handlePublish()}>
              {action === "publish" ? "Publishing..." : "Publish"}
            </Button>
          )}
          {video.status === "published" && (
            <Button
              variant="outline"
              size="sm"
              disabled={Boolean(action)}
              onClick={() => void handleUnpublish()}
            >
              {action === "unpublish" ? "Unpublishing..." : "Unpublish"}
            </Button>
          )}
          {canRetryProcessing(video) && (
            <Button
              variant="outline"
              size="sm"
              disabled={Boolean(action)}
              onClick={() => void handleRetry()}
            >
              <HugeiconsIcon icon={Refresh01Icon} strokeWidth={1.8} />
              {action === "retry" ? "Retrying..." : "Retry processing"}
            </Button>
          )}
          <Button
            variant="destructive"
            size="icon-sm"
            aria-label="Delete video"
            title="Delete"
            disabled={Boolean(action)}
            onClick={() => setDeleteOpen(true)}
          >
            <HugeiconsIcon icon={Delete01Icon} strokeWidth={1.8} />
          </Button>
        </div>
      </div>

      {actionError && (
        <div role="alert" className="mb-4">
          <div className="flex items-start gap-2 border border-destructive/30 bg-destructive/5 p-3 text-xs text-destructive">
            <HugeiconsIcon icon={Alert02Icon} strokeWidth={1.8} className="mt-0.5 size-4 shrink-0" />
            <p>{actionError}</p>
          </div>
        </div>
      )}

      {video.moderationStatus !== "approved" && (
        <div className="mb-4 border border-amber-200 bg-amber-50 p-3 text-xs text-amber-800 dark:border-amber-900 dark:bg-amber-950/40 dark:text-amber-400">
          <p className="font-medium">{MODERATION_LABELS[video.moderationStatus]}</p>
          <p className="mt-0.5">
            This video is not publicly available and cannot be published until a
            reviewer approves it.
            {video.moderationReason ? ` ${video.moderationReason}` : ""}
          </p>
        </div>
      )}

      {video.status === "failed" && (
        <div className="mb-4 border border-destructive/30 bg-destructive/5 p-3 text-xs text-destructive">
          <p className="font-medium">Processing failed</p>
          <p className="mt-0.5">
            {video.processingErrorMessage ?? "The media engine could not process this file."}
            {video.processingErrorCode ? ` (${video.processingErrorCode})` : ""}
          </p>
          <p className="mt-1 text-muted-foreground">
            You can retry processing without uploading the file again.
          </p>
        </div>
      )}

      <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_340px]">
        <div className="space-y-4">
          <div className="border border-border bg-card p-3">
            {canPreview && playback?.playbackUrl ? (
              <VideoPlayer
                playbackUrl={playback.playbackUrl}
                poster={playback.thumbnailUrl ?? video.thumbnailUrl}
              />
            ) : canPreview && playbackError ? (
              <div className="flex aspect-video w-full flex-col items-center justify-center gap-2 border border-border bg-muted/30 text-center">
                <HugeiconsIcon
                  icon={PlayCircleIcon}
                  strokeWidth={1.5}
                  className="size-7 text-muted-foreground"
                />
                <p className="text-xs text-muted-foreground">{playbackError}</p>
                <Button
                  size="sm"
                  variant="outline"
                  disabled={Boolean(action)}
                  onClick={retryPreview}
                >
                  Refresh preview
                </Button>
              </div>
            ) : (
              <div className="flex aspect-video w-full flex-col items-center justify-center gap-2 border border-border bg-muted/30 text-center">
                <HugeiconsIcon
                  icon={PlayCircleIcon}
                  strokeWidth={1.5}
                  className="size-7 text-muted-foreground"
                />
                <p className="text-xs text-muted-foreground">
                  {canPreview
                    ? action === "playback"
                      ? "Loading preview..."
                      : "Preparing preview..."
                    : "Preview becomes available after processing finishes."}
                </p>
              </div>
            )}
          </div>

          <section className="border border-border bg-card p-4" aria-labelledby="details-heading">
            <h3 id="details-heading" className="text-sm font-semibold">
              Details
            </h3>
            <dl className="mt-2 divide-y divide-border">
              <DetailRow label="Description" value={video.description || "No description"} />
              <DetailRow label="Category" value={categoryName ?? "No category"} />
              <DetailRow label="Language" value={video.language} />
              <DetailRow
                label="Tags"
                value={
                  video.tags.length > 0 ? (
                    <div className="flex flex-wrap gap-1">
                      {video.tags.map((tag) => (
                        <span key={tag.id} className="border border-border px-1.5 py-0.5 text-[11px]">
                          {tag.name}
                        </span>
                      ))}
                    </div>
                  ) : (
                    "No tags"
                  )
                }
              />
              <DetailRow label="Visibility" value={VIDEO_VISIBILITY_LABELS[video.visibility]} />
              <DetailRow label="Duration" value={formatDuration(video.durationSeconds)} />
              <DetailRow label="Created" value={formatDateTime(video.createdAt)} />
              <DetailRow label="Last updated" value={formatDateTime(video.updatedAt)} />
              <DetailRow
                label="Published"
                value={video.publishedAt ? formatDateTime(video.publishedAt) : "Not published"}
              />
              <DetailRow
                label="Views"
                value={`${video.viewCount.toLocaleString()} views · ${video.likeCount.toLocaleString()} likes`}
              />
            </dl>
          </section>
        </div>

        <aside className="space-y-4">
          <section className="border border-border bg-card p-4" aria-labelledby="timeline-heading">
            <h3 id="timeline-heading" className="text-sm font-semibold">
              Processing
            </h3>
            <p className="mt-1 text-[11px] text-muted-foreground">
              {isProcessing(video.status)
                ? "Status updates automatically. Processing can include transcoding, thumbnail generation, and streaming formats."
                : "Only server-confirmed steps are shown as complete."}
            </p>
            <div className="mt-3">
              <Timeline video={video} />
            </div>
            {isProcessing(video.status) && (
              <Separator className="my-2" />
            )}
            {isProcessing(video.status) && (
              <p className="text-[11px] text-muted-foreground" aria-live="polite">
                Waiting for the media pipeline to report progress...
              </p>
            )}
          </section>

          <section className="border border-border bg-card p-4">
            <h3 className="text-sm font-semibold">Publishing</h3>
            <p className="mt-1 text-[11px] text-muted-foreground">
              {video.status === "published"
                ? "This video is published. Unpublishing returns it to private and removes public discovery."
                : video.status === "ready" && video.moderationStatus === "approved"
                  ? "Processing is complete. Publish when you are ready."
                  : video.status === "failed"
                    ? "Retry processing before publishing."
                    : video.status === "ready"
                      ? "This video is awaiting moderation approval."
                      : "Publishing becomes available when processing completes."}
            </p>
            {video.visibility === "unlisted" && video.status !== "published" && (
              <p className="mt-2 text-[11px] text-muted-foreground">
                Unlisted videos keep direct-link access after publishing but never
                appear in public discovery.
              </p>
            )}
          </section>
        </aside>
      </div>

      <ConfirmationDialog
        open={deleteOpen}
        onOpenChange={(open) => {
          if (!open && action !== "delete") setDeleteOpen(false)
        }}
        title="Delete this video?"
        description={`"${video.title}" and its media will be permanently removed. This cannot be undone.`}
        confirmLabel={action === "delete" ? "Deleting..." : "Delete"}
        confirmDisabled={action === "delete"}
        onConfirm={() => void handleDelete()}
      />
    </div>
  )
}

export default VideoDetail
