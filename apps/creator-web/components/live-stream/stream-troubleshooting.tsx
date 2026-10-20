"use client"

import { HugeiconsIcon } from "@hugeicons/react"
import { HelpCircleIcon, ChevronDownIcon } from "@hugeicons/core-free-icons"
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"

const TIPS: { title: string; body: string }[] = [
  {
    title: "OBS says “Failed to connect”",
    body: "Your machine's IP address may have changed. Copy the current Server URL from the OBS connection panel again.",
  },
  {
    title: "Status stays “Waiting” after starting OBS",
    body: "Use Service “Custom” with the exact Server URL and full Stream Key (including ?token=). Click Apply, then Start Streaming.",
  },
  {
    title: "Status is Live but the preview shows no video",
    body: "The first HLS segments take a few seconds to appear after OBS connects. Wait briefly, then use Retry on the preview.",
  },
  {
    title: "Preview keeps buffering",
    body: "Check the machine running OBS for encoder overload (dropped frames) and your uplink bandwidth.",
  },
]

export function StreamTroubleshooting() {
  return (
    <Collapsible>
      <Card size="sm">
        <CardHeader>
          <CardTitle>Troubleshooting</CardTitle>
          <CardDescription>Common OBS and preview issues.</CardDescription>
        </CardHeader>
        <CardContent>
          <CollapsibleTrigger className="flex w-full items-center justify-between gap-2 border border-border px-2.5 py-2 text-xs font-medium transition-colors hover:bg-muted/40">
            <span className="inline-flex items-center gap-1.5">
              <HugeiconsIcon icon={HelpCircleIcon} strokeWidth={2} className="size-3.5" />
              View fixes
            </span>
            <HugeiconsIcon icon={ChevronDownIcon} strokeWidth={2} className="size-3.5 text-muted-foreground" />
          </CollapsibleTrigger>
          <CollapsibleContent>
            <ul className="mt-3 space-y-3">
              {TIPS.map((tip) => (
                <li key={tip.title} className="space-y-0.5">
                  <p className="text-xs font-medium">{tip.title}</p>
                  <p className="text-[11px] text-muted-foreground">{tip.body}</p>
                </li>
              ))}
            </ul>
          </CollapsibleContent>
        </CardContent>
      </Card>
    </Collapsible>
  )
}
