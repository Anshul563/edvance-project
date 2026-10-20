"use client"

import { useCallback, useEffect, useRef, useState, type FormEvent } from "react"
import Hls from "hls.js"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  LiveStreaming02Icon,
  Copy01Icon,
  CheckmarkCircle02Icon,
  Alert02Icon,
  Video01Icon,
} from "@hugeicons/core-free-icons"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Label } from "@/components/ui/label"
import { EmptyState } from "@/components/empty-state"
import { ConfirmationDialog } from "@/components/confirmation-dialog"
import { cn } from "@/lib/utils"
import {
  createLiveStream,
  endLiveStream,
  listMyLiveStreams,
  regenerateLiveStreamKey,
  type CreatedLiveStream,
  type LiveStream,
} from "@/components/api-client"

const POLL_MS = 5000

const STATUS_STYLES: Record<LiveStream["status"], string> = {
  waiting: "border-amber-500/40 bg-amber-500/10 text-amber-600 dark:text-amber-400",
  live: "border-red-500/40 bg-red-500/10 text-red-600 dark:text-red-400",
  ended: "border-border bg-muted text-muted-foreground",
}

const STATUS_DOT: Record<LiveStream["status"], string> = {
  waiting: "bg-amber-500",
  live: "bg-red-500 animate-pulse",
  ended: "bg-muted-foreground",
}

export default function LiveStudioPage() {
  const [streams, setStreams] = useState<LiveStream[]>([])
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [rawStreamKey, setRawStreamKey] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)
  const [copied, setCopied] = useState<string | null>(null)

  const [showCreate, setShowCreate] = useState(false)
  const [title, setTitle] = useState("")
  const [description, setDescription] = useState("")
  const [creating, setCreating] = useState(false)

  const [ending, setEnding] = useState(false)
  const [regenerating, setRegenerating] = useState(false)
  const [showEndConfirm, setShowEndConfirm] = useState(false)

  // Preview failure is remembered per stream id so effects never need to
  // reset state synchronously.
  const [failedPreviewId, setFailedPreviewId] = useState<string | null>(null)
  const videoRef = useRef<HTMLVideoElement | null>(null)
  const hlsRef = useRef<Hls | null>(null)

  const selected = streams.find((stream) => stream.id === selectedId) ?? null
  const previewFailed = selected != null && failedPreviewId === selected.id

  const handleResult = useCallback(
    (result: Awaited<ReturnType<typeof listMyLiveStreams>>, options?: { silent?: boolean }) => {
      if (!result.success) {
        if (!options?.silent) setError(result.error?.message ?? "Failed to load streams")
        return
      }
      const items = result.data.items ?? []
      setStreams(items)
      setError(null)
      setLoading(false)
      setSelectedId((current) => {
        if (current && items.some((stream) => stream.id === current)) return current
        const active = items.find((stream) => stream.status !== "ended")
        return active?.id ?? items[0]?.id ?? null
      })
    },
    []
  )

  const refresh = useCallback(
    async (options?: { silent?: boolean }) => {
      handleResult(await listMyLiveStreams(), options)
    },
    [handleResult]
  )

  useEffect(() => {
    let stale = false
    void listMyLiveStreams().then((result) => {
      if (!stale) handleResult(result)
    })
    return () => {
      stale = true
    }
  }, [handleResult])

  // Poll the selected stream's status while it can still change.
  useEffect(() => {
    if (!selectedId || selected?.status === "ended") return
    const timer = setInterval(() => {
      void refresh({ silent: true })
    }, POLL_MS)
    return () => clearInterval(timer)
  }, [selectedId, selected?.status, refresh])

  // Attach an HLS player to the preview area when the stream is live.
  useEffect(() => {
    const video = videoRef.current
    if (!video || !selected || selected.status !== "live") {
      if (hlsRef.current) {
        hlsRef.current.destroy()
        hlsRef.current = null
      }
      video?.removeAttribute("src")
      return
    }

    const source = selected.playbackUrl
    const streamId = selected.id

    if (Hls.isSupported()) {
      const hls = new Hls({
        lowLatencyMode: true,
        // Stay a few segments behind the live edge so small encoder
        // hiccups don't cause repeated rebuffering.
        liveSyncDurationCount: 3,
        liveMaxLatencyDurationCount: 12,
        // Retry generously before giving up on the preview.
        manifestLoadingMaxRetry: 6,
        levelLoadingMaxRetry: 6,
        fragLoadingMaxRetry: 6,
        backBufferLength: 60,
      })
      hlsRef.current = hls

      // Canonical hls.js recovery: transient network/media errors retry
      // once instead of instantly failing the preview.
      let recovered = false
      hls.on(Hls.Events.ERROR, (_event, data) => {
        if (!data.fatal) return
        if (!recovered) {
          recovered = true
          if (data.type === Hls.ErrorTypes.NETWORK_ERROR) {
            hls.startLoad()
            return
          }
          if (data.type === Hls.ErrorTypes.MEDIA_ERROR) {
            hls.recoverMediaError()
            return
          }
        }
        setFailedPreviewId(streamId)
      })
      hls.loadSource(source)
      hls.attachMedia(video)
      return () => {
        hls.destroy()
        hlsRef.current = null
      }
    }

    if (video.canPlayType("application/vnd.apple.mpegurl")) {
      video.src = source
      const onError = () => setFailedPreviewId(streamId)
      video.addEventListener("error", onError)
      return () => video.removeEventListener("error", onError)
    }

    setFailedPreviewId(streamId)
    return undefined
  }, [selected?.id, selected?.status, selected?.playbackUrl, selected])

  async function handleCreate(event: FormEvent) {
    event.preventDefault()
    setCreating(true)
    setActionError(null)

    const result = await createLiveStream({ title, description })
    setCreating(false)

    if (!result.success) {
      setActionError(result.error?.message ?? "Failed to create stream")
      return
    }

    const created: CreatedLiveStream = result.data
    setRawStreamKey(created.streamKey)
    setTitle("")
    setDescription("")
    setShowCreate(false)
    await refresh({ silent: true })
    setSelectedId(created.id)
  }

  async function handleEnd() {
    if (!selected) return
    setEnding(true)
    setActionError(null)

    const result = await endLiveStream(selected.id)
    setEnding(false)
    setShowEndConfirm(false)

    if (!result.success) {
      setActionError(result.error?.message ?? "Failed to end stream")
      return
    }

    setRawStreamKey(null)
    await refresh({ silent: true })
  }

  async function handleRegenerate() {
    if (!selected) return
    setRegenerating(true)
    setActionError(null)

    const result = await regenerateLiveStreamKey(selected.id)
    setRegenerating(false)

    if (!result.success) {
      setActionError(result.error?.message ?? "Failed to regenerate stream key")
      return
    }

    setRawStreamKey(result.data.streamKey)
    await refresh({ silent: true })
  }

  async function copyValue(value: string, label: string) {
    try {
      await navigator.clipboard.writeText(value)
      setCopied(label)
      setTimeout(() => setCopied(null), 2000)
    } catch {
      setActionError("Clipboard is not available in this browser")
    }
  }

  return (
    <div className="p-4 sm:p-6 max-w-7xl mx-auto space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 className="text-lg font-semibold">Live Studio</h2>
          <p className="text-xs text-muted-foreground mt-0.5 max-w-xl">
            Broadcast a live class from OBS. Create a stream, copy the RTMP
            details into OBS, then start streaming — the status below updates
            when OBS actually connects.
          </p>
        </div>
        <Button onClick={() => setShowCreate((value) => !value)} className="gap-1.5">
          <HugeiconsIcon icon={LiveStreaming02Icon} strokeWidth={2} className="size-4" />
          Create Live Stream
        </Button>
      </div>

      {error && (
        <div className="flex items-center gap-2 border border-destructive/40 bg-destructive/10 px-3 py-2 text-xs text-destructive">
          <HugeiconsIcon icon={Alert02Icon} strokeWidth={2} className="size-4 shrink-0" />
          <span>{error}</span>
          <Button variant="ghost" size="xs" className="ml-auto" onClick={() => void refresh()}>
            Retry
          </Button>
        </div>
      )}

      {actionError && (
        <div className="flex items-center gap-2 border border-destructive/40 bg-destructive/10 px-3 py-2 text-xs text-destructive">
          <HugeiconsIcon icon={Alert02Icon} strokeWidth={2} className="size-4 shrink-0" />
          <span>{actionError}</span>
          <button
            type="button"
            className="ml-auto text-[10px] uppercase tracking-wide hover:underline"
            onClick={() => setActionError(null)}
          >
            Dismiss
          </button>
        </div>
      )}

      {showCreate && (
        <form onSubmit={handleCreate} className="border border-border p-4 space-y-4">
          <p className="text-xs font-medium">New live stream</p>
          <div className="grid gap-3 sm:grid-cols-2">
            <div className="space-y-1.5">
              <Label htmlFor="live-title">Title</Label>
              <Input
                id="live-title"
                value={title}
                onChange={(event) => setTitle(event.target.value)}
                placeholder="Algebra II — Quadratic equations"
                required
                maxLength={200}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="live-description">Description</Label>
              <Textarea
                id="live-description"
                value={description}
                onChange={(event) => setDescription(event.target.value)}
                placeholder="What this class covers (optional)"
                rows={2}
                maxLength={2000}
              />
            </div>
          </div>
          <div className="flex gap-2">
            <Button type="submit" disabled={creating || title.trim() === ""}>
              {creating ? "Creating…" : "Create Live Stream"}
            </Button>
            <Button type="button" variant="outline" onClick={() => setShowCreate(false)}>
              Cancel
            </Button>
          </div>
        </form>
      )}

      {loading ? (
        <div className="border border-border p-8 text-center text-xs text-muted-foreground">
          Loading your streams…
        </div>
      ) : streams.length === 0 ? (
        <EmptyState
          icon={LiveStreaming02Icon}
          title="No live streams yet"
          description="Create a live stream to get an RTMP server URL and stream key for OBS."
          action={{ label: "Create Live Stream", onClick: () => setShowCreate(true) }}
        />
      ) : (
        <div className="grid gap-4 lg:grid-cols-[minmax(0,2fr)_minmax(0,1fr)]">
          {/* Selected stream detail + OBS panel + preview */}
          <div className="space-y-4">
            {selected && (
              <>
                <div className="border border-border p-4 space-y-3">
                  <div className="flex flex-wrap items-start justify-between gap-2">
                    <div>
                      <h3 className="text-sm font-semibold">{selected.title}</h3>
                      {selected.description && (
                        <p className="text-xs text-muted-foreground mt-0.5">
                          {selected.description}
                        </p>
                      )}
                    </div>
                    <Badge variant="outline" className={cn("gap-1.5", STATUS_STYLES[selected.status])}>
                      <span className={cn("size-1.5 rounded-full", STATUS_DOT[selected.status])} />
                      {selected.statusLabel}
                    </Badge>
                  </div>

                  <p className="text-[11px] text-muted-foreground">
                    Last updated{" "}
                    {new Date(selected.updatedAt).toLocaleTimeString()}
                    {selected.startedAt && (
                      <> · started {new Date(selected.startedAt).toLocaleTimeString()}</>
                    )}
                  </p>

                  {selected.status === "waiting" && (
                    <p className="text-xs text-amber-600 dark:text-amber-400">
                      Waiting for OBS — the status flips to Live only after the
                      media server sees your broadcast.
                    </p>
                  )}
                  {selected.status === "live" && (
                    <p className="text-xs text-red-600 dark:text-red-400">
                      OBS is connected and broadcasting.
                    </p>
                  )}
                  {selected.status === "ended" && (
                    <p className="text-xs text-muted-foreground">
                      This stream has ended. Create a new stream to broadcast again.
                    </p>
                  )}

                  {selected.status !== "ended" && (
                    <div className="flex flex-wrap gap-2">
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => setShowEndConfirm(true)}
                        disabled={ending}
                        className="text-destructive hover:bg-destructive/10"
                      >
                        {ending ? "Ending…" : "End Stream"}
                      </Button>
                    </div>
                  )}
                </div>

                {selected.status !== "ended" && (
                  <div className="border border-border p-4 space-y-4">
                    <div>
                      <p className="text-xs font-medium">OBS connection details</p>
                      <p className="text-[11px] text-muted-foreground mt-0.5">
                        In OBS: Settings → Stream → Service: Custom → paste the
                        Server and Stream Key below.
                      </p>
                    </div>

                    <div className="space-y-1.5">
                      <Label className="text-[11px] text-muted-foreground">RTMP server URL</Label>
                      <div className="flex items-center gap-2">
                        <code className="flex-1 truncate border border-border bg-muted/50 px-2 py-1.5 text-xs">
                          {selected.rtmpUrl}
                        </code>
                        <Button
                          variant="outline"
                          size="sm"
                          onClick={() => void copyValue(selected.rtmpUrl, "rtmp")}
                        >
                          <HugeiconsIcon
                            icon={copied === "rtmp" ? CheckmarkCircle02Icon : Copy01Icon}
                            strokeWidth={2}
                            className="size-3.5"
                          />
                          {copied === "rtmp" ? "Copied" : "Copy"}
                        </Button>
                      </div>
                    </div>

                    <div className="space-y-1.5">
                      <Label className="text-[11px] text-muted-foreground">Stream key</Label>
                      {rawStreamKey ? (
                        <div className="flex items-center gap-2">
                          <code className="flex-1 truncate border border-border bg-muted/50 px-2 py-1.5 text-xs">
                            {rawStreamKey}
                          </code>
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => void copyValue(rawStreamKey, "key")}
                          >
                            <HugeiconsIcon
                              icon={copied === "key" ? CheckmarkCircle02Icon : Copy01Icon}
                              strokeWidth={2}
                              className="size-3.5"
                            />
                            {copied === "key" ? "Copied" : "Copy"}
                          </Button>
                        </div>
                      ) : (
                        <div className="space-y-2">
                          <p className="text-xs text-muted-foreground">
                            For security the key is shown only once at creation.
                            Hint: <code className="text-foreground">{selected.streamKeyHint}…</code>
                          </p>
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => void handleRegenerate()}
                            disabled={regenerating}
                          >
                            {regenerating ? "Regenerating…" : "Regenerate stream key"}
                          </Button>
                        </div>
                      )}
                      <p className="text-[11px] text-muted-foreground">
                        Students never receive this key. If it leaks, regenerate it.
                      </p>
                    </div>

                    <ol className="list-decimal space-y-1 pl-4 text-xs text-muted-foreground">
                      <li>Open OBS → Settings → Stream.</li>
                      <li>
                        Service: <span className="text-foreground">Custom…</span>
                      </li>
                      <li>
                        Server: <span className="text-foreground">{selected.rtmpUrl}</span>
                      </li>
                      <li>Stream Key: paste the key above (including any “?token=” part).</li>
                      <li>Click Apply, then Start Streaming in OBS.</li>
                    </ol>
                  </div>
                )}

                <div className="border border-border p-4 space-y-3">
                  <p className="text-xs font-medium">Stream preview</p>
                  {selected.status === "live" ? (
                    previewFailed ? (
                      <div className="flex h-48 flex-col items-center justify-center gap-2 border border-border bg-muted/30 text-center">
                        <HugeiconsIcon
                          icon={Alert02Icon}
                          strokeWidth={1.7}
                          className="size-5 text-muted-foreground"
                        />
                        <p className="text-xs text-muted-foreground">
                          Live preview is not supported in this browser.
                        </p>
                        <a
                          href={selected.playbackUrl}
                          target="_blank"
                          rel="noreferrer"
                          className="text-xs text-primary underline-offset-2 hover:underline"
                        >
                          Open HLS playback URL
                        </a>
                      </div>
                    ) : (
                      <video
                        ref={videoRef}
                        controls
                        muted
                        playsInline
                        className="aspect-video w-full border border-border bg-black"
                      />
                    )
                  ) : (
                    <div className="flex h-48 flex-col items-center justify-center gap-2 border border-dashed border-border bg-muted/20 text-center">
                      <HugeiconsIcon
                        icon={Video01Icon}
                        strokeWidth={1.7}
                        className="size-5 text-muted-foreground"
                      />
                      <p className="text-xs text-muted-foreground">
                        {selected.status === "waiting"
                          ? "Preview appears here once OBS is live."
                          : "This stream has ended."}
                      </p>
                    </div>
                  )}
                </div>
              </>
            )}
          </div>

          {/* Stream list */}
          <div className="border border-border">
            <div className="border-b border-border px-4 py-2.5">
              <p className="text-xs font-medium">Your streams</p>
            </div>
            <ul className="divide-y divide-border">
              {streams.map((stream) => (
                <li key={stream.id}>
                  <button
                    type="button"
                    onClick={() => {
                      setSelectedId(stream.id)
                      setRawStreamKey(null)
                    }}
                    className={cn(
                      "flex w-full items-center justify-between gap-2 px-4 py-3 text-left text-xs transition-colors hover:bg-muted/40",
                      stream.id === selectedId && "bg-muted/50",
                    )}
                  >
                    <div className="min-w-0">
                      <p className="truncate font-medium">{stream.title}</p>
                      <p className="mt-0.5 text-[11px] text-muted-foreground">
                        {new Date(stream.createdAt).toLocaleString()}
                      </p>
                    </div>
                    <Badge variant="outline" className={cn("shrink-0 gap-1.5", STATUS_STYLES[stream.status])}>
                      <span className={cn("size-1.5 rounded-full", STATUS_DOT[stream.status])} />
                      {stream.statusLabel}
                    </Badge>
                  </button>
                </li>
              ))}
            </ul>
          </div>
        </div>
      )}

      <ConfirmationDialog
        open={showEndConfirm}
        onOpenChange={setShowEndConfirm}
        title="End this live stream?"
        description="Students will stop receiving the stream and the OBS key stops working immediately. This cannot be undone."
        confirmLabel={ending ? "Ending…" : "End Stream"}
        onConfirm={() => void handleEnd()}
      />
    </div>
  )
}
