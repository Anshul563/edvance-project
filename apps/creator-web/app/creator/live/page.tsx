"use client"

import { useCallback, useEffect, useState, type FormEvent } from "react"
import { HugeiconsIcon } from "@hugeicons/react"
import {
  LiveStreaming02Icon,
  Alert02Icon,
  PlusSignIcon,
} from "@hugeicons/core-free-icons"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Textarea } from "@/components/ui/textarea"
import { Label } from "@/components/ui/label"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { EmptyState } from "@/components/empty-state"
import { cn } from "@/lib/utils"
import {
  createLiveStream,
  endLiveStream,
  listMyLiveStreams,
  regenerateLiveStreamKey,
  type CreatedLiveStream,
  type LiveStream,
} from "@/components/api-client"
import { StreamPreviewPlayer } from "@/components/live-stream/stream-preview-player"
import { StreamStatusBadge } from "@/components/live-stream/stream-status-badge"
import { StreamHealthPanel } from "@/components/live-stream/stream-health-panel"
import { ObsConnectionPanel } from "@/components/live-stream/obs-connection-panel"
import { StreamActions } from "@/components/live-stream/stream-actions"
import { StreamPreviewSkeleton } from "@/components/live-stream/stream-preview-skeleton"
import { StreamTroubleshooting } from "@/components/live-stream/stream-troubleshooting"

const POLL_MS = 5000

export default function LiveStudioPage() {
  const [streams, setStreams] = useState<LiveStream[]>([])
  const [selectedId, setSelectedId] = useState<string | null>(null)
  const [rawStreamKey, setRawStreamKey] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)

  const [showCreate, setShowCreate] = useState(false)
  const [title, setTitle] = useState("")
  const [description, setDescription] = useState("")
  const [creating, setCreating] = useState(false)

  const [ending, setEnding] = useState(false)
  const [regenerating, setRegenerating] = useState(false)

  const selected = streams.find((stream) => stream.id === selectedId) ?? null

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

  // Poll while the selected stream can still change.
  useEffect(() => {
    if (!selectedId || selected?.status === "ended") return
    const timer = setInterval(() => {
      void refresh({ silent: true })
    }, POLL_MS)
    return () => clearInterval(timer)
  }, [selectedId, selected?.status, refresh])

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

  function selectStream(streamId: string) {
    setSelectedId(streamId)
    setRawStreamKey(null)
  }

  return (
    <div className="p-4 sm:p-6 max-w-7xl mx-auto space-y-6">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold tracking-tight">Live Studio</h1>
          <p className="text-sm text-muted-foreground mt-1 max-w-xl">
            Broadcast a live class from OBS. Copy the RTMP details into OBS, start
            streaming, and the status updates automatically once the media server
            receives your broadcast.
          </p>
        </div>
        <Button onClick={() => setShowCreate((value) => !value)} className="gap-1.5">
          <HugeiconsIcon
            icon={showCreate ? LiveStreaming02Icon : PlusSignIcon}
            strokeWidth={2}
            className="size-4"
          />
          {showCreate ? "Close" : "Create Live Stream"}
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
        <StreamPreviewSkeleton />
      ) : streams.length === 0 ? (
        <EmptyState
          icon={LiveStreaming02Icon}
          title="No live streams yet"
          description="Create a live stream to get an RTMP server URL and stream key for OBS."
          action={{ label: "Create Live Stream", onClick: () => setShowCreate(true) }}
        />
      ) : (
        <div className="grid gap-6 lg:grid-cols-[minmax(0,2fr)_minmax(320px,1fr)] items-start">
          {/* Main column: preview + stream details */}
          <div className="space-y-4 min-w-0">
            {selected && (
              <>
                <StreamPreviewPlayer
                  key={`${selected.id}:${selected.status}`}
                  playbackUrl={selected.playbackUrl}
                  status={selected.status}
                  statusLabel={selected.statusLabel}
                />

                <div className="space-y-1.5">
                  <div className="flex flex-wrap items-center gap-2">
                    <h2 className="text-lg font-semibold tracking-tight">{selected.title}</h2>
                    <StreamStatusBadge
                      status={selected.status}
                      statusLabel={selected.statusLabel}
                    />
                  </div>
                  {selected.description && (
                    <p className="text-sm text-muted-foreground">{selected.description}</p>
                  )}
                </div>
              </>
            )}
          </div>

          {/* Side column: OBS panel, health, actions, troubleshooting, list */}
          <div className="space-y-4 min-w-0">
            {selected && (
              <>
                {rawStreamKey && (
                  <div className="border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-xs text-amber-700 dark:text-amber-400">
                    Copy the new stream key into OBS now — it is shown only once.
                  </div>
                )}
                <ObsConnectionPanel
                  key={selected.id}
                  stream={selected}
                  rawStreamKey={rawStreamKey}
                  regenerating={regenerating}
                  onRegenerate={() => void handleRegenerate()}
                  onClipboardError={() =>
                    setActionError("Clipboard is not available in this browser")
                  }
                />
                <StreamHealthPanel
                  stream={selected}
                  onClipboardError={() =>
                    setActionError("Clipboard is not available in this browser")
                  }
                />
                <StreamActions
                  stream={selected}
                  ending={ending}
                  regenerating={regenerating}
                  onEnd={() => void handleEnd()}
                  onRegenerate={() => void handleRegenerate()}
                />
              </>
            )}

            <StreamTroubleshooting />

            <Card size="sm">
              <CardHeader>
                <CardTitle>Your streams</CardTitle>
              </CardHeader>
              <CardContent>
                <ul className="divide-y divide-border -mx-(--card-spacing)">
                  {streams.map((stream) => (
                    <li key={stream.id}>
                      <button
                        type="button"
                        onClick={() => selectStream(stream.id)}
                        className={cn(
                          "flex w-full items-center justify-between gap-2 px-(--card-spacing) py-2.5 text-left text-xs transition-colors hover:bg-muted/40",
                          stream.id === selectedId && "bg-muted/50",
                        )}
                      >
                        <div className="min-w-0">
                          <p className="truncate font-medium">{stream.title}</p>
                          <p className="mt-0.5 text-[11px] text-muted-foreground">
                            {new Date(stream.createdAt).toLocaleString()}
                          </p>
                        </div>
                        <StreamStatusBadge
                          status={stream.status}
                          statusLabel={stream.statusLabel}
                          className="shrink-0"
                        />
                      </button>
                    </li>
                  ))}
                </ul>
              </CardContent>
            </Card>
          </div>
        </div>
      )}
    </div>
  )
}
