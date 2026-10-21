"use client"

import { useEffect, useRef, useState, type DragEvent } from "react"
import { useRouter } from "next/navigation"
import Link from "next/link"
import { toast } from "sonner"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  Alert02Icon,
  ArrowLeft01Icon,
  ArrowRight01Icon,
  Cancel01Icon,
  CheckmarkCircle01Icon,
  Delete01Icon,
  FileUploadIcon,
  InformationCircleIcon,
  Upload01Icon,
  Video01Icon,
} from "@hugeicons/core-free-icons"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Textarea } from "@/components/ui/textarea"
import { Checkbox } from "@/components/ui/checkbox"
import { Progress, ProgressLabel } from "@/components/ui/progress"
import { cn } from "@/lib/utils"
import {
  ACCEPTED_VIDEO_EXTENSIONS,
  ACCEPTED_VIDEO_TYPES,
  MAX_VIDEO_SIZE_BYTES,
  VIDEO_LANGUAGES,
  formatBytes,
  type VideoCategory,
  type VideoVisibility,
} from "./video-types"
import {
  completeUpload,
  createVideo,
  deleteMediaAsset,
  initiateUpload,
  listCategories,
  updateVideo,
  uploadToStorage,
  UploadRequestError,
} from "./video-api"

type Step = 1 | 2 | 3

type Phase =
  | "idle"
  | "creating"
  | "initiating"
  | "linking"
  | "uploading"
  | "finalizing"
  | "error"
  | "cancelled"

const PHASE_LABELS: Record<Phase, string> = {
  idle: "Ready to upload",
  creating: "Creating video draft",
  initiating: "Preparing secure upload",
  linking: "Linking upload to your video",
  uploading: "Uploading to storage",
  finalizing: "Confirming upload",
  error: "Upload paused",
  cancelled: "Upload cancelled",
}

const STEP_LABELS: { step: Step; label: string }[] = [
  { step: 1, label: "Select video" },
  { step: 2, label: "Video details" },
  { step: 3, label: "Upload" },
]

function validateFile(file: File): string | null {
  if (!ACCEPTED_VIDEO_TYPES.includes(file.type as (typeof ACCEPTED_VIDEO_TYPES)[number])) {
    return "Choose an MP4, WebM, MOV, or MKV video."
  }
  if (file.size <= 0) return "The selected file is empty."
  if (file.size > MAX_VIDEO_SIZE_BYTES) {
    return `The file is larger than ${formatBytes(MAX_VIDEO_SIZE_BYTES)}.`
  }
  if (file.name.length > 255) return "The filename is too long."
  return null
}

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

function ErrorNotice({ message }: { message: string }) {
  return (
    <div
      role="alert"
      className="flex items-start gap-2 border border-destructive/30 bg-destructive/5 p-3 text-xs text-destructive"
    >
      <HugeiconsIcon icon={Alert02Icon} strokeWidth={1.8} className="mt-0.5 size-4 shrink-0" />
      <p>{message}</p>
    </div>
  )
}

export function UploadWizard() {
  const router = useRouter()

  const [step, setStep] = useState<Step>(1)
  const [file, setFile] = useState<File | null>(null)
  const [fileError, setFileError] = useState<string | null>(null)
  const [dragActive, setDragActive] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)

  const [categories, setCategories] = useState<VideoCategory[]>([])
  const [title, setTitle] = useState("")
  const [description, setDescription] = useState("")
  const [categorySlug, setCategorySlug] = useState("")
  const [tagsInput, setTagsInput] = useState("")
  const [language, setLanguage] = useState("en")
  const [visibility, setVisibility] = useState<VideoVisibility>("private")
  const [educational, setEducational] = useState(false)
  const [rights, setRights] = useState(false)
  const [detailsError, setDetailsError] = useState<string | null>(null)

  const [phase, setPhase] = useState<Phase>("idle")
  const [progress, setProgress] = useState(0)
  const [error, setError] = useState<string | null>(null)

  const [videoId, setVideoId] = useState<string | null>(null)
  const [assetId, setAssetId] = useState<string | null>(null)
  const abortRef = useRef<AbortController | null>(null)
  const busyRef = useRef(false)

  const busy =
    phase === "creating" ||
    phase === "initiating" ||
    phase === "linking" ||
    phase === "uploading" ||
    phase === "finalizing"

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
    if (phase !== "uploading" && phase !== "finalizing") return
    const handler = (event: BeforeUnloadEvent) => {
      event.preventDefault()
      event.returnValue = ""
    }
    window.addEventListener("beforeunload", handler)
    return () => window.removeEventListener("beforeunload", handler)
  }, [phase])

  useEffect(() => {
    return () => abortRef.current?.abort()
  }, [])

  function chooseFile(next: File | null) {
    if (!next) {
      setFile(null)
      setFileError(null)
      return
    }
    const validationError = validateFile(next)
    setFile(next)
    setFileError(validationError)
    setError(null)
  }

  function handleDrop(event: DragEvent<HTMLDivElement>) {
    event.preventDefault()
    setDragActive(false)
    const dropped = event.dataTransfer.files?.[0]
    if (dropped) chooseFile(dropped)
  }

  function goToDetails() {
    if (!file || fileError) return
    setStep(2)
  }

  function validateDetails(): string | null {
    const trimmed = title.trim()
    if (trimmed.length < 3) return "Add a title of at least 3 characters."
    if (trimmed.length > 200) return "Keep the title under 200 characters."
    if (description.length > 5000) return "Keep the description under 5,000 characters."
    if (parseTags(tagsInput).length > 10) return "Use at most 10 tags."
    if (!educational || !rights) {
      return "Confirm that the video is educational and that you hold the rights to upload it."
    }
    return null
  }

  function goToUpload() {
    const validationError = validateDetails()
    setDetailsError(validationError)
    if (validationError) return
    setError(null)
    setPhase("idle")
    setStep(3)
  }

  async function startUpload() {
    if (!file || busyRef.current) return
    const validationError = validateDetails() ?? fileError
    if (validationError) {
      setError(validationError)
      setPhase("error")
      return
    }

    busyRef.current = true
    setError(null)
    setProgress(0)

    let currentVideoId = videoId

    if (!currentVideoId) {
      setPhase("creating")
      const created = await createVideo({
        title: title.trim(),
        description: description.trim() || undefined,
        visibility,
        language,
        categorySlug: categorySlug || undefined,
        tags: parseTags(tagsInput),
      })
      if (!created.success) {
        busyRef.current = false
        setPhase("error")
        setError(created.error?.message ?? "The video could not be created.")
        return
      }
      currentVideoId = created.data.id
      setVideoId(currentVideoId)
    }

    // A retry after a failed upload needs a fresh presigned URL. Remove
    // the abandoned asset so bytes are not orphaned.
    if (assetId) {
      await deleteMediaAsset(assetId)
      setAssetId(null)
    }

    setPhase("initiating")
    const session = await initiateUpload({
      file,
      educationalConfirmed: educational,
      rightsConfirmed: rights,
    })
    if (!session.success) {
      busyRef.current = false
      setPhase("error")
      setError(session.error?.message ?? "The upload session could not be created.")
      return
    }

    setAssetId(session.data.asset.id)

    setPhase("linking")
    const linked = await updateVideo(currentVideoId, {
      mediaAssetId: session.data.asset.id,
    })
    if (!linked.success) {
      busyRef.current = false
      setPhase("error")
      setError(
        linked.error?.message ??
          "The upload was prepared but could not be linked to your video."
      )
      return
    }

    setPhase("uploading")
    const controller = new AbortController()
    abortRef.current = controller

    try {
      await uploadToStorage(
        session.data.uploadUrl,
        file,
        session.data.contentType,
        {
          signal: controller.signal,
          onProgress: setProgress,
        }
      )
    } catch (uploadError) {
      busyRef.current = false
      abortRef.current = null
      if (controller.signal.aborted) {
        setPhase("cancelled")
        return
      }
      setPhase("error")
      setError(
        uploadError instanceof UploadRequestError
          ? `${uploadError.message}${
              uploadError.expired
                ? " Retrying will request a new secure upload link."
                : ""
            }`
          : "The upload failed. Check your connection and try again."
      )
      return
    }

    setPhase("finalizing")
    const completed = await completeUpload(session.data.asset.id)
    abortRef.current = null
    busyRef.current = false

    if (!completed.success) {
      setPhase("error")
      setError(
        completed.error?.message ??
          "The file uploaded, but the server could not confirm it. Try again."
      )
      return
    }

    setPhase("idle")
    toast.success("Upload complete", {
      description: "Processing has started. You can publish when it is ready.",
    })
    router.push(`/creator/videos/${currentVideoId}?uploaded=1`)
  }

  function cancelUpload() {
    abortRef.current?.abort()
  }

  function resetSelection() {
    if (busy) return
    setFile(null)
    setFileError(null)
    setProgress(0)
    setError(null)
    setPhase("idle")
    if (inputRef.current) inputRef.current.value = ""
  }

  return (
    <div className="mx-auto max-w-4xl p-4 sm:p-6">
      <div className="mb-6 flex items-center gap-2">
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label="Back to videos"
          onClick={() => router.push("/creator/videos")}
        >
          <HugeiconsIcon icon={ArrowLeft01Icon} strokeWidth={1.8} />
        </Button>
        <div>
          <h2 className="text-lg font-semibold">Upload Video</h2>
          <p className="mt-0.5 text-xs text-muted-foreground">
            Upload a standalone educational video. It stays private until you
            publish it.
          </p>
        </div>
      </div>

      <ol className="mb-6 flex flex-wrap items-center gap-2 text-xs" aria-label="Upload steps">
        {STEP_LABELS.map((item, index) => {
          const state =
            step === item.step ? "current" : step > item.step ? "complete" : "upcoming"
          return (
            <li key={item.step} className="flex items-center gap-2">
              <span
                aria-current={state === "current" ? "step" : undefined}
                className={cn(
                  "flex size-5 items-center justify-center border text-[11px] font-medium",
                  state === "current" && "border-primary bg-primary text-primary-foreground",
                  state === "complete" && "border-emerald-300 bg-emerald-50 text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/40 dark:text-emerald-400",
                  state === "upcoming" && "border-border text-muted-foreground"
                )}
              >
                {state === "complete" ? (
                  <HugeiconsIcon icon={CheckmarkCircle01Icon} strokeWidth={2} className="size-3.5" />
                ) : (
                  item.step
                )}
              </span>
              <span className={cn(state === "current" ? "font-medium" : "text-muted-foreground")}>
                {item.label}
              </span>
              {index < STEP_LABELS.length - 1 && (
                <span className="mx-1 h-px w-6 bg-border" aria-hidden="true" />
              )}
            </li>
          )
        })}
      </ol>

      {step === 1 && (
        <section className="border border-border bg-card p-4 sm:p-6" aria-labelledby="select-video-heading">
          <h3 id="select-video-heading" className="text-sm font-semibold">
            Select a video file
          </h3>
          <p className="mt-1 text-xs text-muted-foreground">
            MP4, WebM, MOV, or MKV. Up to {formatBytes(MAX_VIDEO_SIZE_BYTES)}.
          </p>

          <div
            role="button"
            tabIndex={0}
            aria-label="Choose a video file or drop one here"
            onClick={() => inputRef.current?.click()}
            onKeyDown={(event) => {
              if (event.key === "Enter" || event.key === " ") {
                event.preventDefault()
                inputRef.current?.click()
              }
            }}
            onDragOver={(event) => {
              event.preventDefault()
              setDragActive(true)
            }}
            onDragLeave={() => setDragActive(false)}
            onDrop={handleDrop}
            className={cn(
              "mt-4 flex min-h-44 cursor-pointer flex-col items-center justify-center gap-2 border border-dashed px-4 py-8 text-center transition-colors",
              dragActive ? "border-primary bg-primary/5" : "border-border hover:bg-muted/40"
            )}
          >
            <HugeiconsIcon
              icon={FileUploadIcon}
              strokeWidth={1.5}
              className="size-8 text-muted-foreground"
            />
            <p className="text-sm font-medium">Drag and drop your video here</p>
            <p className="text-xs text-muted-foreground">or click to browse your files</p>
          </div>
          <input
            ref={inputRef}
            type="file"
            accept={`${ACCEPTED_VIDEO_TYPES.join(",")},${ACCEPTED_VIDEO_EXTENSIONS}`}
            className="sr-only"
            onChange={(event) => chooseFile(event.target.files?.[0] ?? null)}
          />

          {file && (
            <div className="mt-4 flex flex-wrap items-center gap-3 border border-border p-3">
              <span className="flex size-9 items-center justify-center bg-muted">
                <HugeiconsIcon icon={Video01Icon} strokeWidth={1.7} className="size-4" />
              </span>
              <div className="min-w-0 flex-1">
                <p className="truncate text-xs font-medium">{file.name}</p>
                <p className="mt-0.5 text-[11px] text-muted-foreground">
                  {formatBytes(file.size)} · {file.type || "unknown type"}
                </p>
              </div>
              <Button
                variant="ghost"
                size="sm"
                onClick={(event) => {
                  event.stopPropagation()
                  resetSelection()
                }}
              >
                <HugeiconsIcon icon={Delete01Icon} strokeWidth={1.8} />
                Remove
              </Button>
            </div>
          )}

          {fileError && <div className="mt-3"><ErrorNotice message={fileError} /></div>}

          <div className="mt-6 flex justify-end">
            <Button size="sm" disabled={!file || Boolean(fileError)} onClick={goToDetails}>
              Continue
              <HugeiconsIcon icon={ArrowRight01Icon} strokeWidth={1.8} />
            </Button>
          </div>
        </section>
      )}

      {step === 2 && (
        <section className="border border-border bg-card p-4 sm:p-6" aria-labelledby="details-heading">
          <h3 id="details-heading" className="text-sm font-semibold">
            Video details
          </h3>
          <p className="mt-1 text-xs text-muted-foreground">
            This information appears wherever the video is shared.
          </p>

          <div className="mt-4 grid gap-4">
            <div className="flex flex-col gap-1">
              <Label htmlFor="video-title" className="text-xs font-medium">
                Title <span className="text-destructive">*</span>
              </Label>
              <Input
                id="video-title"
                value={title}
                maxLength={200}
                placeholder="e.g. Introduction to linear algebra"
                onChange={(event) => setTitle(event.target.value)}
                aria-invalid={Boolean(detailsError) && title.trim().length < 3}
              />
              <p className="text-[11px] text-muted-foreground">
                {title.trim().length}/200 characters (minimum 3)
              </p>
            </div>

            <div className="flex flex-col gap-1">
              <Label htmlFor="video-description" className="text-xs font-medium">
                Description
              </Label>
              <Textarea
                id="video-description"
                value={description}
                rows={4}
                maxLength={5000}
                placeholder="What will learners get from this video?"
                onChange={(event) => setDescription(event.target.value)}
              />
            </div>

            <div className="grid gap-4 sm:grid-cols-2">
              <div className="flex flex-col gap-1">
                <Label htmlFor="video-category" className="text-xs font-medium">
                  Category
                </Label>
                <select
                  id="video-category"
                  value={categorySlug}
                  onChange={(event) => setCategorySlug(event.target.value)}
                  className="h-8 w-full border border-border bg-background px-2 text-xs outline-none focus:border-ring focus:ring-1 focus:ring-ring/50"
                >
                  <option value="">No category</option>
                  {categories.map((category) => (
                    <option key={category.id} value={category.slug}>
                      {category.name}
                    </option>
                  ))}
                </select>
                {categories.length === 0 && (
                  <p className="text-[11px] text-muted-foreground">
                    No categories are configured yet. You can upload without one.
                  </p>
                )}
              </div>

              <div className="flex flex-col gap-1">
                <Label htmlFor="video-language" className="text-xs font-medium">
                  Language
                </Label>
                <select
                  id="video-language"
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
              <Label htmlFor="video-tags" className="text-xs font-medium">
                Tags
              </Label>
              <Input
                id="video-tags"
                value={tagsInput}
                placeholder="algebra, mathematics, beginner"
                onChange={(event) => setTagsInput(event.target.value)}
              />
              <p className="text-[11px] text-muted-foreground">
                Comma-separated, up to 10 tags.
              </p>
            </div>

            <fieldset className="flex flex-col gap-1">
              <legend className="text-xs font-medium">Visibility</legend>
              <div className="mt-1 flex flex-col gap-2">
                {(
                  [
                    ["private", "Private — only you can see it"],
                    ["unlisted", "Unlisted — anyone with the link can watch after publishing"],
                    ["public", "Public — discoverable after publishing"],
                  ] as const
                ).map(([value, label]) => (
                  <label key={value} className="flex items-center gap-2 text-xs">
                    <input
                      type="radio"
                      name="visibility"
                      value={value}
                      checked={visibility === value}
                      onChange={() => setVisibility(value)}
                      className="accent-[var(--primary)]"
                    />
                    {label}
                  </label>
                ))}
              </div>
            </fieldset>

            <div className="space-y-3 border border-border p-3">
              <label htmlFor="educational-confirm" className="flex items-start gap-2 text-xs">
                <Checkbox
                  id="educational-confirm"
                  checked={educational}
                  onCheckedChange={(checked) => setEducational(Boolean(checked))}
                  className="mt-0.5"
                />
                <span>
                  This video is educational content intended for learners.
                  <span className="text-destructive"> *</span>
                </span>
              </label>
              <label htmlFor="rights-confirm" className="flex items-start gap-2 text-xs">
                <Checkbox
                  id="rights-confirm"
                  checked={rights}
                  onCheckedChange={(checked) => setRights(Boolean(checked))}
                  className="mt-0.5"
                />
                <span>
                  I own or have permission to upload this video and its audio.
                  <span className="text-destructive"> *</span>
                </span>
              </label>
            </div>
          </div>

          {detailsError && <div className="mt-4"><ErrorNotice message={detailsError} /></div>}

          <div className="mt-6 flex justify-between gap-2">
            <Button variant="outline" size="sm" onClick={() => setStep(1)}>
              <HugeiconsIcon icon={ArrowLeft01Icon} strokeWidth={1.8} />
              Back
            </Button>
            <Button size="sm" onClick={goToUpload}>
              Continue
              <HugeiconsIcon icon={ArrowRight01Icon} strokeWidth={1.8} />
            </Button>
          </div>
        </section>
      )}

      {step === 3 && file && (
        <section className="border border-border bg-card p-4 sm:p-6" aria-labelledby="upload-heading">
          <h3 id="upload-heading" className="text-sm font-semibold">
            Upload to storage
          </h3>
          <p className="mt-1 text-xs text-muted-foreground">
            Your file uploads directly to secure object storage. It never passes
            through the API gateway.
          </p>

          <div className="mt-4 flex flex-wrap items-center gap-3 border border-border p-3">
            <span className="flex size-9 items-center justify-center bg-muted">
              <HugeiconsIcon icon={Video01Icon} strokeWidth={1.7} className="size-4" />
            </span>
            <div className="min-w-0 flex-1">
              <p className="truncate text-xs font-medium">{title.trim() || "Untitled video"}</p>
              <p className="mt-0.5 text-[11px] text-muted-foreground">
                {file.name} · {formatBytes(file.size)}
              </p>
            </div>
            {phase === "uploading" && (
              <Button variant="outline" size="sm" onClick={cancelUpload}>
                <HugeiconsIcon icon={Cancel01Icon} strokeWidth={1.8} />
                Cancel
              </Button>
            )}
          </div>

          {(busy || progress > 0 || phase === "error" || phase === "cancelled") && (
            <div className="mt-4">
              <Progress value={progress}>
                <ProgressLabel>{PHASE_LABELS[phase]}</ProgressLabel>
                <span className="ml-auto text-xs tabular-nums text-muted-foreground">
                  {progress}%
                </span>
              </Progress>
            </div>
          )}

          <ul className="mt-4 space-y-2 text-xs" aria-label="Upload progress">
            <li className="flex items-center gap-2">
              <HugeiconsIcon
                icon={videoId ? CheckmarkCircle01Icon : InformationCircleIcon}
                strokeWidth={1.8}
                className={cn("size-4", videoId ? "text-emerald-600" : "text-muted-foreground")}
              />
              {videoId ? "Video draft created" : "Video draft will be created"}
            </li>
            <li className="flex items-center gap-2">
              <HugeiconsIcon
                icon={phase === "uploading" || phase === "finalizing" ? CheckmarkCircle01Icon : InformationCircleIcon}
                strokeWidth={1.8}
                className={cn(
                  "size-4",
                  phase === "uploading" || phase === "finalizing"
                    ? "text-emerald-600"
                    : "text-muted-foreground"
                )}
              />
              Secure upload link ready
            </li>
            <li className="flex items-center gap-2">
              <HugeiconsIcon
                icon={phase === "finalizing" ? CheckmarkCircle01Icon : InformationCircleIcon}
                strokeWidth={1.8}
                className={cn(
                  "size-4",
                  phase === "finalizing" ? "text-emerald-600" : "text-muted-foreground"
                )}
              />
              File uploaded to storage
            </li>
            <li className="flex items-center gap-2">
              <HugeiconsIcon
                icon={InformationCircleIcon}
                strokeWidth={1.8}
                className="size-4 text-muted-foreground"
              />
              Processing starts after the server confirms the upload
            </li>
          </ul>

          {error && <div className="mt-4"><ErrorNotice message={error} /></div>}

          {phase === "cancelled" && (
            <p className="mt-4 text-xs text-muted-foreground">
              The upload was cancelled. Nothing was marked as uploaded.
            </p>
          )}

          <div className="mt-6 flex flex-wrap justify-between gap-2">
            <Button
              variant="outline"
              size="sm"
              disabled={busy}
              onClick={() => setStep(2)}
            >
              <HugeiconsIcon icon={ArrowLeft01Icon} strokeWidth={1.8} />
              Back
            </Button>
            <Button size="sm" disabled={busy} onClick={() => void startUpload()}>
              <HugeiconsIcon icon={Upload01Icon} strokeWidth={1.8} />
              {phase === "error" || phase === "cancelled" ? "Retry upload" : "Start upload"}
            </Button>
          </div>
        </section>
      )}

      <p className="mt-4 text-[11px] text-muted-foreground">
        Files are uploaded directly to private object storage. Publishing is a
        separate step and is only available after processing completes.{" "}
        <Link href="/creator/videos" className="text-primary hover:underline">
          Back to My Videos
        </Link>
      </p>
    </div>
  )
}

export default UploadWizard
