"use client"

import { useCallback, useEffect, useMemo, useState } from "react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import { toast } from "sonner"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  Alert02Icon,
  ArrowLeft01Icon,
  ArrowRight01Icon,
  Delete01Icon,
  PencilEdit01Icon,
  Refresh01Icon,
  Upload01Icon,
  Video01Icon,
} from "@hugeicons/core-free-icons"
import { Button } from "@/components/ui/button"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Skeleton } from "@/components/ui/skeleton"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { EmptyState } from "@/components/empty-state"
import { ConfirmationDialog } from "@/components/confirmation-dialog"
import { SearchFilter } from "@/components/search-filter"
import {
  canPublish,
  canRetryProcessing,
  formatDateTime,
  formatDuration,
  type Video,
  type VideoCategory,
  type VideoStatus,
  type VideoVisibility,
} from "./video-types"
import {
  deleteMediaAsset,
  deleteVideo,
  getMyCreator,
  listCategories,
  listVideos,
  onboardCreator,
  publishVideo,
  resetCreatorCache,
  retryProcessing,
  unpublishVideo,
} from "./video-api"
import { ModerationBadge, VideoStatusBadge } from "./video-status"

const PAGE_SIZE = 10

const STATUS_OPTIONS: { value: VideoStatus | ""; label: string }[] = [
  { value: "", label: "All statuses" },
  { value: "draft", label: "Draft" },
  { value: "upload_pending", label: "Waiting for upload" },
  { value: "uploaded", label: "Uploaded" },
  { value: "processing", label: "Processing" },
  { value: "ready", label: "Ready to publish" },
  { value: "published", label: "Published" },
  { value: "failed", label: "Processing failed" },
  { value: "archived", label: "Archived" },
]

const VISIBILITY_OPTIONS: { value: VideoVisibility | ""; label: string }[] = [
  { value: "", label: "All visibility" },
  { value: "public", label: "Public" },
  { value: "unlisted", label: "Unlisted" },
  { value: "private", label: "Private" },
]

const SORT_OPTIONS = [
  { value: "newest", label: "Newest" },
  { value: "oldest", label: "Oldest" },
  { value: "popular", label: "Most viewed" },
] as const

function FilterSelect({
  label,
  value,
  onChange,
  children,
}: {
  label: string
  value: string
  onChange: (value: string) => void
  children: React.ReactNode
}) {
  return (
    <label className="flex items-center gap-1.5 text-xs text-muted-foreground">
      <span className="sr-only">{label}</span>
      <select
        aria-label={label}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="h-8 border border-border bg-background px-2 text-xs text-foreground outline-none focus:border-ring focus:ring-1 focus:ring-ring/50"
      >
        {children}
      </select>
    </label>
  )
}

function Thumbnail({ video }: { video: Video }) {
  return (
    <div className="relative flex h-12 w-20 shrink-0 items-center justify-center overflow-hidden border border-border bg-muted">
      {video.thumbnailUrl ? (
        // Thumbnails are backend-provided URLs (R2/CDN), not Next-optimized.
        // eslint-disable-next-line @next/next/no-img-element
        <img
          src={video.thumbnailUrl}
          alt=""
          className="h-full w-full object-cover"
          loading="lazy"
        />
      ) : (
        <HugeiconsIcon
          icon={Video01Icon}
          strokeWidth={1.5}
          className="size-5 text-muted-foreground"
        />
      )}
      {video.durationSeconds ? (
        <span className="absolute bottom-0.5 right-0.5 bg-black/75 px-1 text-[10px] text-white">
          {formatDuration(video.durationSeconds)}
        </span>
      ) : null}
    </div>
  )
}

function RowActions({
  video,
  pending,
  onPublish,
  onUnpublish,
  onRetry,
  onDelete,
}: {
  video: Video
  pending: boolean
  onPublish: (video: Video) => void
  onUnpublish: (video: Video) => void
  onRetry: (video: Video) => void
  onDelete: (video: Video) => void
}) {
  return (
    <div className="flex items-center justify-end gap-1">
      <Link href={`/creator/videos/${video.id}`} aria-label={`View ${video.title}`}>
        <Button variant="ghost" size="sm" title="View">
          View
        </Button>
      </Link>
      <Link href={`/creator/videos/${video.id}/edit`} aria-label={`Edit ${video.title}`}>
        <Button variant="ghost" size="icon-sm" title="Edit">
          <HugeiconsIcon icon={PencilEdit01Icon} strokeWidth={1.8} />
        </Button>
      </Link>
      {canPublish(video) && (
        <Button
          variant="outline"
          size="sm"
          disabled={pending}
          onClick={() => onPublish(video)}
        >
          Publish
        </Button>
      )}
      {video.status === "published" && (
        <Button
          variant="outline"
          size="sm"
          disabled={pending}
          onClick={() => onUnpublish(video)}
        >
          Unpublish
        </Button>
      )}
      {canRetryProcessing(video) && (
        <Button
          variant="outline"
          size="sm"
          disabled={pending}
          onClick={() => onRetry(video)}
        >
          <HugeiconsIcon icon={Refresh01Icon} strokeWidth={1.8} />
          Retry
        </Button>
      )}
      <Button
        variant="ghost"
        size="icon-sm"
        aria-label={`Delete ${video.title}`}
        title="Delete"
        disabled={pending}
        onClick={() => onDelete(video)}
      >
        <HugeiconsIcon
          icon={Delete01Icon}
          strokeWidth={1.8}
          className="text-destructive"
        />
      </Button>
    </div>
  )
}

export function VideoLibrary() {
  const router = useRouter()

  const [creatorId, setCreatorId] = useState<string | null>(null)
  const [creatorError, setCreatorError] = useState<string | null>(null)
  const [creatorErrorStatus, setCreatorErrorStatus] = useState<number | null>(null)
  const [creatorLoading, setCreatorLoading] = useState(true)
  const [channelName, setChannelName] = useState("")
  const [handle, setHandle] = useState("")
  const [description, setDescription] = useState("")
  const [onboarding, setOnboarding] = useState(false)
  const [onboardError, setOnboardError] = useState<string | null>(null)

  const [categories, setCategories] = useState<VideoCategory[]>([])
  const [videos, setVideos] = useState<Video[]>([])
  const [total, setTotal] = useState(0)
  const [hasNext, setHasNext] = useState(false)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [reloadToken, setReloadToken] = useState(0)

  const [searchInput, setSearchInput] = useState("")
  const [query, setQuery] = useState("")
  const [status, setStatus] = useState<VideoStatus | "">("")
  const [visibility, setVisibility] = useState<VideoVisibility | "">("")
  const [category, setCategory] = useState("")
  const [sort, setSort] = useState<"newest" | "oldest" | "popular">("newest")
  const [page, setPage] = useState(1)

  const [pendingId, setPendingId] = useState<string | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<Video | null>(null)
  const [deleting, setDeleting] = useState(false)

  const hasFilters = Boolean(query || status || visibility || category)

  useEffect(() => {
    const timer = setTimeout(() => {
      setQuery(searchInput.trim())
      setPage(1)
    }, 300)
    return () => clearTimeout(timer)
  }, [searchInput])

  const loadCreator = useCallback(async () => {
    const result = await getMyCreator()
    if (!result.success) {
      setCreatorError(result.error?.message ?? "Could not load your creator profile.")
      setCreatorErrorStatus(result.error?.status ?? null)
      setCreatorLoading(false)
      return
    }
    setCreatorErrorStatus(null)
    setCreatorId(result.data.id)
    setCreatorLoading(false)
  }, [])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void loadCreator()
  }, [loadCreator])

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

  useEffect(() => {
    if (!creatorId) return

    const controller = new AbortController()
    // The request lifecycle drives these flags; the lint rule cannot see
    // through the memoized fetcher below.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setLoading(true)
    setError(null)

    void listVideos({
      creatorId,
      page,
      limit: PAGE_SIZE,
      status: status || undefined,
      visibility: visibility || undefined,
      category: category || undefined,
      q: query || undefined,
      sort,
      signal: controller.signal,
    }).then((result) => {
      if (controller.signal.aborted) return
      setLoading(false)
      if (!result.success) {
        setError(result.error?.message ?? "Could not load your videos.")
        return
      }
      setVideos(result.data.items ?? [])
      setTotal(result.data.total ?? 0)
      setHasNext(Boolean(result.data.hasNext))
    })

    return () => {
      controller.abort()
    }
  }, [
    creatorId,
    page,
    status,
    visibility,
    category,
    query,
    sort,
    reloadToken,
  ])

  const categoryNames = useMemo(() => {
    const map = new Map<string, string>()
    for (const item of categories) map.set(item.id, item.name)
    return map
  }, [categories])

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  function clearFilters() {
    setSearchInput("")
    setQuery("")
    setStatus("")
    setVisibility("")
    setCategory("")
    setSort("newest")
    setPage(1)
  }

  function reload() {
    setReloadToken((token) => token + 1)
  }

  async function handleOnboard(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (onboarding) return
    setOnboarding(true)
    setOnboardError(null)
    const result = await onboardCreator({
      channelName: channelName.trim(),
      handle: handle.trim(),
      description: description.trim() || undefined,
    })
    setOnboarding(false)
    if (!result.success) {
      setOnboardError(result.error?.message ?? "The creator profile could not be created.")
      return
    }
    setCreatorError(null)
    setCreatorErrorStatus(null)
    setCreatorId(result.data.id)
    setChannelName("")
    setHandle("")
    setDescription("")
  }

  async function runAction(
    video: Video,
    action: () => Promise<{ success: boolean; error?: { message: string } }>,
    successMessage: string
  ) {
    if (pendingId) return
    setPendingId(video.id)
    const result = await action()
    setPendingId(null)
    if (!result.success) {
      toast.error(result.error?.message ?? "The action could not be completed.")
      return
    }
    toast.success(successMessage)
    reload()
  }

  async function handleDelete() {
    const video = deleteTarget
    if (!video || deleting) return
    setDeleting(true)

    const result = await deleteVideo(video.id)
    if (!result.success) {
      setDeleting(false)
      toast.error(result.error?.message ?? "The video could not be deleted.")
      return
    }

    // The media asset is owned by video-service. The metadata delete above
    // is authoritative; cleaning up bytes is best effort.
    if (video.mediaAssetId) {
      const media = await deleteMediaAsset(video.mediaAssetId)
      if (!media.success) {
        toast.warning("Video deleted, but media cleanup will finish in the background.")
      }
    }

    setDeleting(false)
    setDeleteTarget(null)
    toast.success("Video deleted.")
    if (videos.length === 1 && page > 1) {
      setPage((current) => current - 1)
    } else {
      reload()
    }
  }

  if (creatorLoading) {
    return (
      <div className="mx-auto max-w-7xl p-4 sm:p-6">
        <div className="mb-6 space-y-2">
          <Skeleton className="h-6 w-40" />
          <Skeleton className="h-3 w-96 max-w-full" />
        </div>
        <div className="space-y-2">
          {Array.from({ length: 6 }).map((_, index) => (
            <Skeleton key={index} className="h-16 w-full" />
          ))}
        </div>
      </div>
    )
  }

  if (creatorError) {
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
              <h2 className="text-sm font-semibold">
                {creatorErrorStatus === 404
                  ? "Creator profile required"
                  : "Creator service unavailable"}
              </h2>
              <p className="mt-1 text-xs text-muted-foreground">{creatorError}</p>
              <p className="mt-3 text-xs text-muted-foreground">
                {creatorErrorStatus === 404
                  ? "A creator profile is required to own and publish videos. Create your profile below, then try again."
                  : "The creator service could not be reached, so video ownership cannot be verified. Check that the creator service is running, then retry."}
              </p>
              {creatorErrorStatus === 404 && (
                <form className="mt-4 grid gap-3" onSubmit={handleOnboard}>
                  <div className="grid gap-1">
                    <Label htmlFor="creator-channel-name" className="text-xs">
                      Channel name
                    </Label>
                    <Input
                      id="creator-channel-name"
                      value={channelName}
                      maxLength={100}
                      required
                      placeholder="Ada Learns"
                      onChange={(event) => setChannelName(event.target.value)}
                    />
                  </div>
                  <div className="grid gap-1">
                    <Label htmlFor="creator-handle" className="text-xs">
                      Handle
                    </Label>
                    <Input
                      id="creator-handle"
                      value={handle}
                      maxLength={30}
                      required
                      placeholder="ada-learns"
                      onChange={(event) => setHandle(event.target.value)}
                    />
                    <p className="text-[11px] text-muted-foreground">
                      Your public channel address. Letters, numbers, and hyphens.
                    </p>
                  </div>
                  <div className="grid gap-1">
                    <Label htmlFor="creator-description" className="text-xs">
                      Description (optional)
                    </Label>
                    <Input
                      id="creator-description"
                      value={description}
                      maxLength={1000}
                      placeholder="What do you teach?"
                      onChange={(event) => setDescription(event.target.value)}
                    />
                  </div>
                  {onboardError && (
                    <p role="alert" className="text-xs text-destructive">
                      {onboardError}
                    </p>
                  )}
                  <div>
                    <Button type="submit" size="sm" disabled={onboarding}>
                      {onboarding ? "Creating..." : "Create creator profile"}
                    </Button>
                  </div>
                </form>
              )}
              <Button
                className="mt-4"
                size="sm"
                variant={creatorErrorStatus === 404 ? "ghost" : "default"}
                onClick={() => {
                  resetCreatorCache()
                  setCreatorLoading(true)
                  void loadCreator()
                }}
              >
                Try again
              </Button>
            </div>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="mx-auto max-w-7xl p-4 sm:p-6">
      <div className="mb-6 flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 className="text-lg font-semibold">My Videos</h2>
          <p className="mt-1 max-w-2xl text-xs text-muted-foreground">
            Publish individual educational videos without building a full course.
            Upload a video, add its details, then publish it when processing is
            complete.
          </p>
        </div>
        <Link href="/creator/videos/upload">
          <Button size="sm">
            <HugeiconsIcon icon={Upload01Icon} strokeWidth={1.8} className="mr-1.5" />
            Upload Video
          </Button>
        </Link>
      </div>

      <div className="mb-4 flex flex-wrap items-center gap-2">
        <SearchFilter
          placeholder="Search by title..."
          value={searchInput}
          onChange={setSearchInput}
          className="w-full sm:w-56"
        />
        <FilterSelect
          label="Status"
          value={status}
          onChange={(value) => {
            setStatus(value as VideoStatus | "")
            setPage(1)
          }}
        >
          {STATUS_OPTIONS.map((option) => (
            <option key={option.value || "all"} value={option.value}>
              {option.label}
            </option>
          ))}
        </FilterSelect>
        <FilterSelect
          label="Visibility"
          value={visibility}
          onChange={(value) => {
            setVisibility(value as VideoVisibility | "")
            setPage(1)
          }}
        >
          {VISIBILITY_OPTIONS.map((option) => (
            <option key={option.value || "all"} value={option.value}>
              {option.label}
            </option>
          ))}
        </FilterSelect>
        <FilterSelect
          label="Category"
          value={category}
          onChange={(value) => {
            setCategory(value)
            setPage(1)
          }}
        >
          <option value="">All categories</option>
          {categories.map((item) => (
            <option key={item.id} value={item.slug}>
              {item.name}
            </option>
          ))}
        </FilterSelect>
        <FilterSelect
          label="Sort"
          value={sort}
          onChange={(value) => {
            setSort(value as typeof sort)
            setPage(1)
          }}
        >
          {SORT_OPTIONS.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </FilterSelect>
        {hasFilters && (
          <Button variant="ghost" size="sm" onClick={clearFilters}>
            Clear filters
          </Button>
        )}
      </div>

      {error ? (
        <div role="alert" className="border border-border bg-card p-6">
          <div className="flex items-start gap-3">
            <HugeiconsIcon
              icon={Alert02Icon}
              strokeWidth={1.8}
              className="mt-0.5 size-5 shrink-0 text-destructive"
            />
            <div>
              <h3 className="text-sm font-semibold">Could not load videos</h3>
              <p className="mt-1 text-xs text-muted-foreground">{error}</p>
              <Button className="mt-4" size="sm" variant="outline" onClick={reload}>
                Retry
              </Button>
            </div>
          </div>
        </div>
      ) : loading ? (
        <div className="space-y-2">
          {Array.from({ length: 6 }).map((_, index) => (
            <Skeleton key={index} className="h-16 w-full" />
          ))}
        </div>
      ) : videos.length === 0 ? (
        <EmptyState
          icon={Video01Icon}
          title={hasFilters ? "No videos match your filters" : "No videos yet"}
          description={
            hasFilters
              ? "Try a different search term or clear the filters."
              : "Upload your first standalone educational video. It stays private until you publish it."
          }
          action={
            hasFilters
              ? { label: "Clear filters", onClick: clearFilters }
              : { label: "Upload Video", onClick: () => router.push("/creator/videos/upload") }
          }
        />
      ) : (
        <>
          <div className="hidden border border-border md:block">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Video</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Visibility</TableHead>
                  <TableHead>Updated</TableHead>
                  <TableHead className="text-right">Actions</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {videos.map((video) => (
                  <TableRow key={video.id}>
                    <TableCell>
                      <div className="flex items-center gap-3">
                        <Thumbnail video={video} />
                        <div className="min-w-0">
                          <Link
                            href={`/creator/videos/${video.id}`}
                            className="block truncate text-xs font-medium hover:text-primary"
                          >
                            {video.title}
                          </Link>
                          <div className="mt-1 flex flex-wrap items-center gap-1.5 text-[11px] text-muted-foreground">
                            {video.categoryId && categoryNames.get(video.categoryId) ? (
                              <span>{categoryNames.get(video.categoryId)}</span>
                            ) : null}
                            <span>{formatDuration(video.durationSeconds)}</span>
                            <ModerationBadge status={video.moderationStatus} />
                          </div>
                        </div>
                      </div>
                    </TableCell>
                    <TableCell>
                      <VideoStatusBadge status={video.status} />
                    </TableCell>
                    <TableCell className="text-xs capitalize text-muted-foreground">
                      {video.visibility}
                    </TableCell>
                    <TableCell className="whitespace-nowrap text-xs text-muted-foreground">
                      {formatDateTime(video.updatedAt)}
                    </TableCell>
                    <TableCell>
                      <RowActions
                        video={video}
                        pending={pendingId === video.id}
                        onPublish={(item) =>
                          void runAction(item, () => publishVideo(item.id), "Video published.")
                        }
                        onUnpublish={(item) =>
                          void runAction(
                            item,
                            () => unpublishVideo(item.id),
                            "Video unpublished."
                          )
                        }
                        onRetry={(item) => {
                          if (!item.mediaAssetId) return
                          void runAction(
                            item,
                            () => retryProcessing(item.mediaAssetId as string),
                            "Processing restarted."
                          )
                        }}
                        onDelete={setDeleteTarget}
                      />
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>

          <div className="space-y-3 md:hidden">
            {videos.map((video) => (
              <div key={video.id} className="border border-border bg-card p-3">
                <div className="flex gap-3">
                  <Thumbnail video={video} />
                  <div className="min-w-0 flex-1">
                    <Link
                      href={`/creator/videos/${video.id}`}
                      className="block truncate text-sm font-medium"
                    >
                      {video.title}
                    </Link>
                    <div className="mt-1 flex flex-wrap items-center gap-1.5">
                      <VideoStatusBadge status={video.status} />
                      <ModerationBadge status={video.moderationStatus} />
                    </div>
                    <p className="mt-1 text-[11px] text-muted-foreground">
                      {video.visibility} · {formatDuration(video.durationSeconds)} ·{" "}
                      {formatDateTime(video.updatedAt)}
                    </p>
                  </div>
                </div>
                <div className="mt-3 border-t border-border pt-2">
                  <RowActions
                    video={video}
                    pending={pendingId === video.id}
                    onPublish={(item) =>
                      void runAction(item, () => publishVideo(item.id), "Video published.")
                    }
                    onUnpublish={(item) =>
                      void runAction(item, () => unpublishVideo(item.id), "Video unpublished.")
                    }
                    onRetry={(item) => {
                      if (!item.mediaAssetId) return
                      void runAction(
                        item,
                        () => retryProcessing(item.mediaAssetId as string),
                        "Processing restarted."
                      )
                    }}
                    onDelete={setDeleteTarget}
                  />
                </div>
              </div>
            ))}
          </div>

          <div className="mt-4 flex flex-wrap items-center justify-between gap-2">
            <p className="text-xs text-muted-foreground">
              {total} {total === 1 ? "video" : "videos"} · Page {page} of{" "}
              {totalPages}
            </p>
            <div className="flex gap-1">
              <Button
                variant="outline"
                size="sm"
                disabled={page <= 1 || loading}
                onClick={() => setPage((current) => Math.max(1, current - 1))}
              >
                <HugeiconsIcon icon={ArrowLeft01Icon} strokeWidth={1.8} />
                Previous
              </Button>
              <Button
                variant="outline"
                size="sm"
                disabled={!hasNext || loading}
                onClick={() => setPage((current) => current + 1)}
              >
                Next
                <HugeiconsIcon icon={ArrowRight01Icon} strokeWidth={1.8} />
              </Button>
            </div>
          </div>
        </>
      )}

      <ConfirmationDialog
        open={Boolean(deleteTarget)}
        onOpenChange={(open) => {
          if (!open && !deleting) setDeleteTarget(null)
        }}
        title="Delete this video?"
        description={
          deleteTarget
            ? `"${deleteTarget.title}" and its media will be permanently removed. This cannot be undone.`
            : undefined
        }
        confirmLabel={deleting ? "Deleting..." : "Delete"}
        confirmDisabled={deleting}
        onConfirm={() => void handleDelete()}
      />

      <span className="sr-only" aria-live="polite">
        {loading ? "Loading videos" : `${total} videos loaded`}
      </span>
    </div>
  )
}

export default VideoLibrary
