"use client"

import { useState } from "react"
import { HugeiconsIcon } from "@hugeicons/react"
import { RefreshIcon, Cancel01Icon } from "@hugeicons/core-free-icons"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { ConfirmationDialog } from "@/components/confirmation-dialog"
import { EndStreamDialog } from "./end-stream-dialog"
import type { LiveStream } from "@/components/api-client"

interface StreamActionsProps {
  stream: LiveStream
  ending?: boolean
  regenerating?: boolean
  onEnd: () => void
  onRegenerate: () => void
}

export function StreamActions({
  stream,
  ending = false,
  regenerating = false,
  onEnd,
  onRegenerate,
}: StreamActionsProps) {
  const [showEndConfirm, setShowEndConfirm] = useState(false)
  const [showRegenerateConfirm, setShowRegenerateConfirm] = useState(false)

  return (
    <>
      <Card size="sm">
        <CardHeader>
          <CardTitle>Stream actions</CardTitle>
          <CardDescription>Manage this broadcast.</CardDescription>
        </CardHeader>
        <CardContent className="space-y-2">
          {stream.status !== "ended" ? (
            <>
              <Button
                variant="outline"
                size="sm"
                onClick={() => setShowRegenerateConfirm(true)}
                disabled={regenerating}
                className="w-full justify-start"
              >
                <HugeiconsIcon icon={RefreshIcon} strokeWidth={2} className="size-3.5" />
                {regenerating ? "Regenerating key…" : "Regenerate stream key"}
              </Button>
              <Button
                variant="destructive"
                size="sm"
                onClick={() => setShowEndConfirm(true)}
                disabled={ending}
                className="w-full justify-start"
              >
                <HugeiconsIcon icon={Cancel01Icon} strokeWidth={2} className="size-3.5" />
                {ending ? "Ending stream…" : "End stream"}
              </Button>
              <p className="text-[11px] text-muted-foreground">
                Ending the stream revokes the OBS key and stops student playback.
              </p>
            </>
          ) : (
            <p className="text-xs text-muted-foreground">
              This stream has ended. Create a new stream to broadcast again.
            </p>
          )}
        </CardContent>
      </Card>

      <EndStreamDialog
        open={showEndConfirm}
        onOpenChange={setShowEndConfirm}
        pending={ending}
        onConfirm={onEnd}
      />
      <ConfirmationDialog
        open={showRegenerateConfirm}
        onOpenChange={setShowRegenerateConfirm}
        title="Regenerate the stream key?"
        description="The current OBS key stops working immediately. The new key is shown once — update OBS before the next broadcast."
        confirmLabel={regenerating ? "Regenerating…" : "Regenerate key"}
        confirmDisabled={regenerating}
        onConfirm={onRegenerate}
      />
    </>
  )
}
