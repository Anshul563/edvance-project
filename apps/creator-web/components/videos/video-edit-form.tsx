"use client"

/* eslint-disable @next/next/no-img-element -- thumbnails are inlined as a data URL */

import { useCallback, useEffect, useRef, useState } from "react"
import Link from "next/link"
import { useRouter } from "next/navigation"
import { toast } from "sonner"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  Alert02Icon,
  ArrowLeft01Icon,
  CheckmarkCircle01Icon,
  Delete02Icon,
  Image01Icon,
} from "@hugeicons/core-free-icons"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { Skeleton } from "@/components/ui/skeleton"
import { imageFileToDataUrl } from "@/lib/image"
import {
  VIDEO_LANGUAGES,
  VIDEO_VISIBILITY_LABELS,
  type Video,
  type VideoCategory,
  type VideoVisibility,
} from "./video-types"
import { getVideo, listCategories, updateVideo } from "./video-api"
import { VideoStatusBadge } from "./video-status"

function parseTags(value: string): string[] {
  const seen = new Set<string>()
  const tags: string[] = []
  for (const raw of value.split(",")) {
    const tag = raw.trim()
    if (!tag) continue
    const key = tag.toLowerCase()
    if (seen.has(key)) continue
    seen.add(key)
    tags.push(tag)
  }
  return tags
}

export function VideoEditForm({ videoId }: { videoId: string }) {
  const router = useRouter()

  const [video, setVideo] = useState<Video | null>(null)
  const [categories, setCategories] = useState<VideoCategory[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const [title, setTitle] = useState("")
  const [description, setDescription] = useState("")
  const [thumbnailUrl, setThumbnailUrl] = useState("")
  const [uploadingThumbnail, setUploadingThumbnail] = useState(false)
  const thumbnailInput = useRef<HTMLInputElement>(null)
  const [categorySlug, setCategorySlug] = useState("")
  const [tagsInput, setTagsInput] = useState("")
  const [language, setLanguage] = useState("en")
  const [visibility, setVisibility] = useState<VideoVisibility>("private")
  const [categoryInitialized, setCategoryInitialized] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  const applyVideo = useCallback((next: Video) => {
    setVideo(next)
    setTitle(next.title)
    setDescription(next.description ?? "")
    setThumbnailUrl(next.thumbnailUrl ?? "")
    setTagsInput(next.tags.map((tag) => tag.name).join(", "))
    setLanguage(next.language)
    setVisibility(next.visibility)
  }, [])

  const load = useCallback(async () => {
    const result = await getVideo(videoId)
    if (!result.success) {
      setError(result.error?.message ?? "This video could not be loaded.")
      setLoading(false)
      return
    }
    setError(null)
    applyVideo(result.data)
    setLoading(false)
  }, [applyVideo, videoId])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load()
  }, [load])

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

  const selectedCategorySlug = categoryInitialized
    ? categorySlug
    : (categories.find((category) => category.id === video?.categoryId)?.slug ?? "")

  async function handleThumbnail(
    file: File | undefined,
    input: HTMLInputElement | null
  ) {
    if (!file) return
    setUploadingThumbnail(true)
    try {
      const dataUrl = await imageFileToDataUrl(file, {
        width: 1280,
        aspect: 16 / 9,
      })
      setThumbnailUrl(dataUrl)
    } catch (err) {
      toast.error(
        err instanceof Error ? err.message : "The image could not be processed."
      )
    } finally {
      setUploadingThumbnail(false)
      if (input) input.value = ""
    }
  }

  async function handleSubmit(event: React.FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!video || saving) return

    const trimmedTitle = title.trim()
    if (trimmedTitle.length < 3) {
      setFormError("Add a title of at least 3 characters.")
      return
    }
    if (trimmedTitle.length > 200) {
      setFormError("Keep the title under 200 characters.")
      return
    }
    if (description.length > 5000) {
      setFormError("Keep the description under 5,000 characters.")
      return
    }
    const tags = parseTags(tagsInput)
    if (tags.length > 10) {
      setFormError("Use at most 10 tags.")
      return
    }

    setFormError(null)
    setSaving(true)
    const result = await updateVideo(video.id, {
      title: trimmedTitle,
      description: description.trim(),
      thumbnailUrl: thumbnailUrl.trim(),
      categorySlug,
      tags,
      language,
      visibility,
    })
    setSaving(false)

    if (!result.success) {
      setFormError(result.error?.message ?? "Your changes could not be saved.")
      return
    }

    toast.success("Video details saved.")
    router.push(`/creator/videos/${video.id}`)
  }

  if (loading) {
    return (
      <div className="mx-auto max-w-3xl space-y-3 p-4 sm:p-6">
        <Skeleton className="h-6 w-48" />
        <Skeleton className="h-10 w-full" />
        <Skeleton className="h-32 w-full" />
        <Skeleton className="h-10 w-full" />
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
                    void load()
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

  return (
    <div className="mx-auto max-w-3xl p-4 sm:p-6">
      <div className="mb-5 flex items-start gap-2">
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label="Back to video"
          onClick={() => router.push(`/creator/videos/${video.id}`)}
        >
          <HugeiconsIcon icon={ArrowLeft01Icon} strokeWidth={1.8} />
        </Button>
        <div className="min-w-0">
          <h2 className="truncate text-lg font-semibold">Edit video</h2>
          <div className="mt-1 flex items-center gap-2">
            <VideoStatusBadge status={video.status} />
            <span className="text-[11px] text-muted-foreground">
              Status and moderation are controlled by the platform.
            </span>
          </div>
        </div>
      </div>

      <form
        onSubmit={(event) => void handleSubmit(event)}
        className="space-y-5 border border-border bg-card p-4 sm:p-6"
      >
        <div className="flex flex-col gap-1">
          <Label htmlFor="edit-title" className="text-xs font-medium">
            Title <span className="text-destructive">*</span>
          </Label>
          <Input
            id="edit-title"
            value={title}
            maxLength={200}
            onChange={(event) => setTitle(event.target.value)}
          />
          <p className="text-[11px] text-muted-foreground">
            {title.trim().length}/200 characters (minimum 3)
          </p>
        </div>

        <div className="flex flex-col gap-1">
          <Label htmlFor="edit-description" className="text-xs font-medium">
            Description
          </Label>
          <Textarea
            id="edit-description"
            value={description}
            rows={5}
            maxLength={5000}
            onChange={(event) => setDescription(event.target.value)}
          />
        </div>

        <div className="flex flex-col gap-1">
          <Label className="text-xs font-medium">Thumbnail</Label>
          <div className="flex flex-col gap-3 sm:flex-row sm:items-start">
            <div className="relative aspect-video w-full max-w-xs overflow-hidden border border-border bg-muted">
              {thumbnailUrl ? (
                <img
                  src={thumbnailUrl}
                  alt="Video thumbnail preview"
                  className="size-full object-cover"
                />
              ) : (
                <div className="flex size-full flex-col items-center justify-center gap-1 text-muted-foreground">
                  <HugeiconsIcon icon={Image01Icon} strokeWidth={1.6} className="size-6" />
                  <span className="text-[11px]">No thumbnail yet</span>
                </div>
              )}
            </div>
            <div className="flex flex-col gap-2">
              <input
                ref={thumbnailInput}
                type="file"
                accept="image/jpeg,image/png,image/webp"
                className="hidden"
                onChange={(event) =>
                  void handleThumbnail(event.target.files?.[0], event.target)
                }
              />
              <div className="flex gap-2">
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  disabled={uploadingThumbnail}
                  onClick={() => thumbnailInput.current?.click()}
                >
                  <HugeiconsIcon icon={Image01Icon} strokeWidth={1.8} />
                  {uploadingThumbnail
                    ? "Processing..."
                    : thumbnailUrl
                      ? "Replace image"
                      : "Upload image"}
                </Button>
                {thumbnailUrl && (
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    disabled={uploadingThumbnail}
                    onClick={() => setThumbnailUrl("")}
                  >
                    <HugeiconsIcon icon={Delete02Icon} strokeWidth={1.8} />
                    Remove
                  </Button>
                )}
              </div>
              <p className="text-[11px] text-muted-foreground">
                Recommended 1280×720 (16:9), JPEG or PNG, under 8 MB.
              </p>
            </div>
          </div>
        </div>

        <div className="grid gap-4 sm:grid-cols-2">
          <div className="flex flex-col gap-1">
            <Label htmlFor="edit-category" className="text-xs font-medium">
              Category
            </Label>
            <select
              id="edit-category"
              value={selectedCategorySlug}
              onChange={(event) => {
                setCategorySlug(event.target.value)
                setCategoryInitialized(true)
              }}
              className="h-8 w-full border border-border bg-background px-2 text-xs outline-none focus:border-ring focus:ring-1 focus:ring-ring/50"
            >
              <option value="">No category</option>
              {categories.map((category) => (
                <option key={category.id} value={category.slug}>
                  {category.name}
                </option>
              ))}
            </select>
          </div>

          <div className="flex flex-col gap-1">
            <Label htmlFor="edit-language" className="text-xs font-medium">
              Language
            </Label>
            <select
              id="edit-language"
              value={language}
              onChange={(event) => setLanguage(event.target.value)}
              className="h-8 w-full border border-border bg-background px-2 text-xs outline-none focus:border-ring focus:ring-1 focus:ring-ring/50"
            >
              {VIDEO_LANGUAGES.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
          </div>
        </div>

        <div className="flex flex-col gap-1">
          <Label htmlFor="edit-tags" className="text-xs font-medium">
            Tags
          </Label>
          <Input
            id="edit-tags"
            value={tagsInput}
            onChange={(event) => setTagsInput(event.target.value)}
          />
          <p className="text-[11px] text-muted-foreground">Comma-separated, up to 10 tags.</p>
        </div>

        <div className="flex flex-col gap-1">
          <Label htmlFor="edit-visibility" className="text-xs font-medium">
            Visibility
          </Label>
          <select
            id="edit-visibility"
            value={visibility}
            onChange={(event) => setVisibility(event.target.value as VideoVisibility)}
            className="h-8 w-full border border-border bg-background px-2 text-xs outline-none focus:border-ring focus:ring-1 focus:ring-ring/50"
          >
            {(["private", "unlisted", "public"] as const).map((value) => (
              <option key={value} value={value}>
                {VIDEO_VISIBILITY_LABELS[value]}
              </option>
            ))}
          </select>
          {video.status === "published" && (
            <p className="text-[11px] text-muted-foreground">
              This video is published. Changing visibility affects who can reach it
              immediately; unpublish it to take it off the public surface.
            </p>
          )}
        </div>

        {formError && (
          <div role="alert" className="flex items-start gap-2 border border-destructive/30 bg-destructive/5 p-3 text-xs text-destructive">
            <HugeiconsIcon icon={Alert02Icon} strokeWidth={1.8} className="mt-0.5 size-4 shrink-0" />
            <p>{formError}</p>
          </div>
        )}

        <div className="flex justify-end gap-2">
          <Link href={`/creator/videos/${video.id}`}>
            <Button type="button" variant="outline" size="sm">
              Cancel
            </Button>
          </Link>
          <Button type="submit" size="sm" disabled={saving}>
            <HugeiconsIcon icon={CheckmarkCircle01Icon} strokeWidth={1.8} />
            {saving ? "Saving..." : "Save changes"}
          </Button>
        </div>
      </form>
    </div>
  )
}

export default VideoEditForm
