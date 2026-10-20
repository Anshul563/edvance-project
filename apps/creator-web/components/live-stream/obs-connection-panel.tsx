"use client"

import { useState } from "react"
import { HugeiconsIcon } from "@hugeicons/react"
import { EyeIcon, EyeOffIcon, KeyIcon, Alert01Icon, RefreshIcon } from "@hugeicons/core-free-icons"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Label } from "@/components/ui/label"
import { CopyValueButton } from "./copy-value-button"
import type { LiveStream } from "@/components/api-client"

function maskKey(raw: string): string {
  const prefix = raw.slice(0, 10)
  const suffix = raw.length > 14 ? raw.slice(-4) : ""
  return `${prefix}${"•".repeat(12)}${suffix}`
}

interface ObsConnectionPanelProps {
  stream: LiveStream
  /** Raw key, only available right after create/regenerate. */
  rawStreamKey: string | null
  regenerating?: boolean
  onRegenerate?: () => void
  onClipboardError?: () => void
}

export function ObsConnectionPanel({
  stream,
  rawStreamKey,
  regenerating = false,
  onRegenerate,
  onClipboardError,
}: ObsConnectionPanelProps) {
  const [revealed, setRevealed] = useState(false)

  return (
    <Card size="sm">
      <CardHeader>
        <CardTitle>OBS connection</CardTitle>
        <CardDescription>
          In OBS: Settings → Stream → Service “Custom”, then paste the server and key below.
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="space-y-1.5">
          <Label className="text-[11px] text-muted-foreground">Server (RTMP)</Label>
          <div className="flex items-center gap-2">
            <code className="flex-1 min-w-0 truncate border border-border bg-muted/50 px-2 py-1.5 font-mono text-xs">
              {stream.rtmpUrl}
            </code>
            <CopyValueButton
              value={stream.rtmpUrl}
              onClipboardError={onClipboardError}
            />
          </div>
        </div>

        <div className="space-y-1.5">
          <div className="flex items-center justify-between gap-2">
            <Label className="text-[11px] text-muted-foreground">Stream key</Label>
            {rawStreamKey && (
              <Button
                variant="ghost"
                size="xs"
                onClick={() => setRevealed((value) => !value)}
                aria-label={revealed ? "Hide stream key" : "Reveal stream key"}
              >
                <HugeiconsIcon
                  icon={revealed ? EyeOffIcon : EyeIcon}
                  strokeWidth={2}
                  className="size-3"
                />
                {revealed ? "Hide" : "Reveal"}
              </Button>
            )}
          </div>
          <div className="flex items-center gap-2">
            <code className="flex-1 min-w-0 truncate border border-border bg-muted/50 px-2 py-1.5 font-mono text-xs">
              {rawStreamKey
                ? revealed
                  ? rawStreamKey
                  : maskKey(rawStreamKey)
                : `${stream.streamKeyHint}…`}
            </code>
            <CopyValueButton
              value={rawStreamKey ?? ""}
              disabled={!rawStreamKey}
              onClipboardError={onClipboardError}
            />
          </div>
          {rawStreamKey ? (
            <p className="flex items-start gap-1.5 text-[11px] text-amber-600 dark:text-amber-400">
              <HugeiconsIcon icon={Alert01Icon} strokeWidth={2} className="mt-0.5 size-3 shrink-0" />
              This key is shown only once. Copy it into OBS now — it cannot be viewed again later.
            </p>
          ) : (
            <p className="flex items-start gap-1.5 text-[11px] text-muted-foreground">
              <HugeiconsIcon icon={KeyIcon} strokeWidth={2} className="mt-0.5 size-3 shrink-0" />
              For security the full key is never displayed again after creation.
              Hint: <code className="font-mono text-foreground">{stream.streamKeyHint}…</code>.
              Students never receive this key.
            </p>
          )}
        </div>

        {onRegenerate && stream.status !== "ended" && (
          <Button
            variant="outline"
            size="sm"
            onClick={onRegenerate}
            disabled={regenerating}
            className="w-full"
          >
            <HugeiconsIcon icon={RefreshIcon} strokeWidth={2} className="size-3.5" />
            {regenerating ? "Regenerating…" : "Regenerate stream key"}
          </Button>
        )}

        <ol className="list-decimal space-y-1 pl-4 text-[11px] text-muted-foreground">
          <li>Open OBS → Settings → Stream.</li>
          <li>
            Service: <span className="text-foreground">Custom…</span>
          </li>
          <li>
            Server: <span className="text-foreground font-mono">{stream.rtmpUrl}</span>
          </li>
          <li>Stream Key: paste the key above (including any “?token=” part).</li>
          <li>Click Apply, then Start Streaming in OBS.</li>
        </ol>
      </CardContent>
    </Card>
  )
}
