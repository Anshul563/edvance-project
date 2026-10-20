"use client"

import { HugeiconsIcon } from "@hugeicons/react"
import {
  AntennaIcon,
  CheckmarkCircle02Icon,
  Clock01Icon,
  Cancel01Icon,
  InfoIcon,
  ArrowUpRightIcon,
} from "@hugeicons/core-free-icons"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { cn } from "@/lib/utils"
import { CopyValueButton } from "./copy-value-button"
import type { LiveStream } from "@/components/api-client"

function Row({
  label,
  children,
}: {
  label: string
  children: React.ReactNode
}) {
  return (
    <div className="flex items-center justify-between gap-3 text-xs">
      <span className="text-muted-foreground shrink-0">{label}</span>
      <span className="min-w-0 truncate text-right">{children}</span>
    </div>
  )
}

function formatTimestamp(value?: string): string {
  if (!value) return "—"
  return new Date(value).toLocaleString()
}

interface StreamHealthPanelProps {
  stream: LiveStream
  onClipboardError?: () => void
}

export function StreamHealthPanel({ stream, onClipboardError }: StreamHealthPanelProps) {
  return (
    <Card size="sm">
      <CardHeader>
        <CardTitle>Stream health</CardTitle>
        <CardDescription>Publisher state reported by the media server.</CardDescription>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="flex items-center gap-2 border border-border bg-muted/30 px-2.5 py-2">
          {stream.status === "live" ? (
            <HugeiconsIcon
              icon={CheckmarkCircle02Icon}
              strokeWidth={2}
              className="size-4 shrink-0 text-emerald-600 dark:text-emerald-400"
            />
          ) : stream.status === "waiting" ? (
            <HugeiconsIcon
              icon={AntennaIcon}
              strokeWidth={2}
              className="size-4 shrink-0 text-amber-500"
            />
          ) : (
            <HugeiconsIcon
              icon={Cancel01Icon}
              strokeWidth={2}
              className="size-4 shrink-0 text-muted-foreground"
            />
          )}
          <div className="min-w-0">
            <p
              className={cn(
                "text-xs font-medium",
                stream.status === "live" && "text-emerald-600 dark:text-emerald-400",
                stream.status === "waiting" && "text-amber-600 dark:text-amber-400",
              )}
            >
              {stream.status === "live"
                ? "OBS connected"
                : stream.status === "waiting"
                  ? "Waiting for OBS"
                  : "Broadcast ended"}
            </p>
            <p className="text-[11px] text-muted-foreground">
              {stream.status === "live"
                ? "Media server is receiving your broadcast."
                : stream.status === "waiting"
                  ? "Start streaming in OBS to go live."
                  : "This stream is no longer accepting video."}
            </p>
          </div>
        </div>

        <div className="space-y-2">
          <Row label="Started">{formatTimestamp(stream.startedAt)}</Row>
          <Row label="Last update">{formatTimestamp(stream.updatedAt)}</Row>
          {stream.endedAt && <Row label="Ended">{formatTimestamp(stream.endedAt)}</Row>}
        </div>

        <div className="space-y-1.5 border-t border-border pt-3">
          <div className="flex items-center justify-between gap-2 text-xs">
            <span className="text-muted-foreground">Technical metrics</span>
            <span className="inline-flex items-center gap-1 text-muted-foreground">
              <HugeiconsIcon icon={InfoIcon} strokeWidth={2} className="size-3" />
              Unavailable
            </span>
          </div>
          <p className="text-[11px] text-muted-foreground">
            Bitrate, resolution and viewer counts are not reported by the backend yet.
          </p>
        </div>

        {stream.status !== "ended" && (
          <div className="space-y-1.5 border-t border-border pt-3">
            <p className="text-[11px] text-muted-foreground">Public playback URL (HLS)</p>
            <div className="flex items-center gap-2">
              <code className="flex-1 min-w-0 truncate border border-border bg-muted/50 px-2 py-1.5 font-mono text-xs">
                {stream.playbackUrl}
              </code>
              <CopyValueButton
                value={stream.playbackUrl}
                onClipboardError={onClipboardError}
              />
              <a
                href={stream.playbackUrl}
                target="_blank"
                rel="noreferrer"
                className="inline-flex size-7 shrink-0 items-center justify-center border border-border text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                aria-label="Open playback URL in a new tab"
              >
                <HugeiconsIcon icon={ArrowUpRightIcon} strokeWidth={2} className="size-3.5" />
              </a>
            </div>
          </div>
        )}

        {stream.status === "live" && (
          <p className="flex items-start gap-1.5 text-[11px] text-muted-foreground">
            <HugeiconsIcon icon={Clock01Icon} strokeWidth={2} className="mt-0.5 size-3 shrink-0" />
            Status updates automatically a few seconds after OBS connects or disconnects.
          </p>
        )}
      </CardContent>
    </Card>
  )
}
